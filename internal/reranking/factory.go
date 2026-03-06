package reranking

// NewONNXReranker creates an ONNX-based reranker.
// On Windows, this always returns nil (ONNX runtime unavailable).
// On other platforms, it creates the cross-encoder service.
var NewONNXReranker func(alpha float64) (Reranker, error)
