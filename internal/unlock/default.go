package unlock

// DefaultProvider は実行中のOSで利用できるproviderを返す。
//
// OS固有実装が未対応、または実行条件を満たさない場合も、平文configや
// shell commandを代替経路にせずUnsupportedProviderを返す。
func DefaultProvider() Provider { return defaultProvider() }
