//go:build !windows

package reranking

func init() {
	NewONNXReranker = func(alpha float64) (Reranker, error) {
		cfg := DefaultConfig()
		if alpha > 0 && alpha <= 1 {
			cfg.Alpha = alpha
		}
		return NewService(cfg)
	}
}
