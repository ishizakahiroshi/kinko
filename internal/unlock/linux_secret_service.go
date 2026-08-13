//go:build linux

package unlock

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	secretServiceName          = "org.freedesktop.secrets"
	secretServicePath          = dbus.ObjectPath("/org/freedesktop/secrets")
	secretServiceInterface     = "org.freedesktop.Secret.Service"
	secretCollectionInterface  = "org.freedesktop.Secret.Collection"
	secretItemInterface        = "org.freedesktop.Secret.Item"
	secretAttributeApplication = "application"
	secretAttributeRoute       = "kinko-route"
)

// LinuxProvider は対象desktopのSecret Serviceを使うsession-protected
// adapterである。Secret Serviceが毎回user verificationを要求することは
// providerの契約に含めず、login keyringが既に開いている場合も同じlevelで扱う。
type LinuxProvider struct{}

func NewLinuxProvider() Provider { return &LinuxProvider{} }

func (p *LinuxProvider) Name() string { return "linux-secret-service" }

func (p *LinuxProvider) Level() SecurityLevel { return LevelSessionProtected }

func (p *LinuxProvider) Setup(ctx context.Context, route Route, vaultID, password string, replace bool) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	if password == "" || !validLinuxVaultID(vaultID) {
		return NewProviderError(ErrorCorruptEntry)
	}
	conn, root, session, err := openSecretService(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer closeSecretSession(ctx, conn, session)

	collection, err := defaultSecretCollection(ctx, root)
	if err != nil {
		return err
	}
	unlocked, locked, err := searchSecretItems(ctx, root, route)
	if err != nil {
		return err
	}
	if len(locked) > 0 {
		// This adapter deliberately does not open an untrusted prompt object.
		// The keyring is known to be locked before any UI is shown, so the
		// common service may safely use the explicit master-password fallback.
		return NewProviderError(ErrorUnavailable)
	}
	if len(unlocked) > 0 && !replace {
		return NewProviderError(ErrorAlreadyConfigured)
	}

	payload, err := json.Marshal(linuxCredentialPayload{VaultID: vaultID, Password: password})
	if err != nil {
		return NewProviderError(ErrorCorruptEntry)
	}
	defer zeroBytes(payload)
	properties := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("kinko terminal unlock"),
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
			secretAttributeApplication: "kinko",
			secretAttributeRoute:       route.Key,
		}),
	}
	secret := secretServiceSecret{
		Session:     session,
		Parameters:  []byte{},
		Value:       append([]byte(nil), payload...),
		ContentType: "application/json",
	}
	defer zeroBytes(secret.Value)
	var item, prompt dbus.ObjectPath
	call := dbusObject(conn, collection).CallWithContext(ctx,
		secretCollectionInterface+".CreateItem", 0, properties, secret, replace)
	if err := call.Store(&item, &prompt); err != nil {
		return linuxProviderError(err)
	}
	if prompt != "" {
		return NewProviderError(ErrorUnavailable)
	}
	return nil
}

func (p *LinuxProvider) Unlock(ctx context.Context, route Route) (Credential, error) {
	if err := ctx.Err(); err != nil {
		return Credential{}, providerErrorForContext(err)
	}
	conn, root, session, err := openSecretService(ctx)
	if err != nil {
		return Credential{}, err
	}
	defer conn.Close()
	defer closeSecretSession(ctx, conn, session)

	unlocked, locked, err := searchSecretItems(ctx, root, route)
	if err != nil {
		return Credential{}, err
	}
	if len(unlocked) == 0 {
		if len(locked) > 0 {
			return Credential{}, NewProviderError(ErrorUnavailable)
		}
		return Credential{}, NewProviderError(ErrorNotConfigured)
	}
	if len(unlocked) > 1 {
		return Credential{}, NewProviderError(ErrorCorruptEntry)
	}

	var secret secretServiceSecret
	call := dbusObject(conn, unlocked[0]).CallWithContext(ctx,
		secretItemInterface+".GetSecret", 0, session)
	if err := call.Store(&secret); err != nil {
		return Credential{}, linuxProviderError(err)
	}
	defer zeroBytes(secret.Value)
	var stored linuxCredentialPayload
	if err := json.Unmarshal(secret.Value, &stored); err != nil {
		return Credential{}, NewProviderError(ErrorCorruptEntry)
	}
	if stored.Password == "" || !validLinuxVaultID(stored.VaultID) {
		return Credential{}, NewProviderError(ErrorCorruptEntry)
	}
	return Credential{Password: stored.Password, VaultID: stored.VaultID}, nil
}

func (p *LinuxProvider) Status(ctx context.Context, route Route) Status {
	status := Status{
		Provider:     p.Name(),
		Level:        p.Level(),
		Binding:      BindingSessionProtected,
		Availability: AvailabilityUnavailable,
		Available:    false,
	}
	if err := ctx.Err(); err != nil {
		status.Availability = availabilityForProviderError(providerErrorForContext(err))
		return status
	}
	conn, root, session, err := openSecretService(ctx)
	if err != nil {
		status.Availability = availabilityForProviderError(err)
		return status
	}
	defer conn.Close()
	defer closeSecretSession(ctx, conn, session)
	unlocked, locked, err := searchSecretItems(ctx, root, route)
	if err != nil {
		status.Availability = availabilityForProviderError(err)
		return status
	}
	status.Available = true
	status.Availability = AvailabilityAvailable
	status.Configured = len(unlocked) > 0 || len(locked) > 0
	return status
}

func (p *LinuxProvider) Disable(ctx context.Context, route Route) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	conn, root, session, err := openSecretService(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer closeSecretSession(ctx, conn, session)
	unlocked, locked, err := searchSecretItems(ctx, root, route)
	if err != nil {
		return err
	}
	if len(unlocked) == 0 {
		if len(locked) > 0 {
			return NewProviderError(ErrorUnavailable)
		}
		return NewProviderError(ErrorNotConfigured)
	}
	for _, item := range unlocked {
		var prompt dbus.ObjectPath
		call := dbusObject(conn, item).CallWithContext(ctx, secretItemInterface+".Delete", 0)
		if err := call.Store(&prompt); err != nil {
			return linuxProviderError(err)
		}
		if prompt != "" {
			return NewProviderError(ErrorUnavailable)
		}
	}
	return nil
}

type linuxCredentialPayload struct {
	VaultID  string `json:"vault_id"`
	Password string `json:"password"`
}

type secretServiceSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

func validLinuxVaultID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func openSecretService(ctx context.Context) (*dbus.Conn, dbus.BusObject, dbus.ObjectPath, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, "", providerErrorForContext(err)
	}
	// Do not autolaunch a desktop session bus from a headless/SSH invocation.
	conn, err := dbus.SessionBusPrivateNoAutoStartup()
	if err != nil {
		return nil, nil, "", NewProviderError(ErrorUnavailable)
	}
	root := conn.Object(secretServiceName, secretServicePath)
	var output dbus.Variant
	var session dbus.ObjectPath
	call := root.CallWithContext(ctx, secretServiceInterface+".OpenSession", 0,
		"plain", dbus.MakeVariant(""))
	if err := call.Store(&output, &session); err != nil {
		conn.Close()
		return nil, nil, "", linuxProviderError(err)
	}
	if session == "" {
		conn.Close()
		return nil, nil, "", NewProviderError(ErrorUnavailable)
	}
	return conn, root, session, nil
}

func closeSecretSession(ctx context.Context, conn *dbus.Conn, session dbus.ObjectPath) {
	if session == "" {
		return
	}
	_ = dbusObject(conn, session).CallWithContext(ctx, "org.freedesktop.Secret.Session.Close", 0).Err
}

func defaultSecretCollection(ctx context.Context, root dbus.BusObject) (dbus.ObjectPath, error) {
	var collection dbus.ObjectPath
	call := root.CallWithContext(ctx, secretServiceInterface+".ReadAlias", 0, "default")
	if err := call.Store(&collection); err != nil || collection == "" {
		return "", NewProviderError(ErrorUnavailable)
	}
	return collection, nil
}

func searchSecretItems(ctx context.Context, root dbus.BusObject, route Route) ([]dbus.ObjectPath, []dbus.ObjectPath, error) {
	attributes := map[string]string{
		secretAttributeApplication: "kinko",
		secretAttributeRoute:       route.Key,
	}
	var unlocked, locked []dbus.ObjectPath
	call := root.CallWithContext(ctx, secretServiceInterface+".SearchItems", 0, attributes)
	if err := call.Store(&unlocked, &locked); err != nil {
		return nil, nil, linuxProviderError(err)
	}
	return unlocked, locked, nil
}

func dbusObject(conn *dbus.Conn, path dbus.ObjectPath) dbus.BusObject {
	return conn.Object(secretServiceName, path)
}

func linuxProviderError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return NewProviderError(ErrorTimeout)
	}
	if errors.Is(err, context.Canceled) {
		return NewProviderError(ErrorCanceled)
	}
	var dbusErr *dbus.Error
	if errors.As(err, &dbusErr) {
		switch {
		case strings.Contains(dbusErr.Name, "IsLocked"):
			return NewProviderError(ErrorLocked)
		case strings.Contains(dbusErr.Name, "NoSuchObject"):
			return NewProviderError(ErrorNotConfigured)
		case strings.Contains(dbusErr.Name, "AccessDenied"):
			return NewProviderError(ErrorUnsupported)
		}
	}
	return NewProviderError(ErrorUnavailable)
}
