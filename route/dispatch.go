package route

// DispatchNonBlocking will dispatch in to buf.
// if buf is full, will discard the data
func (r *baseRoute) dispatchNonBlocking(buf chan []byte, in []byte) {
	select {
	case buf <- in:
		r.rm.Buffer.BufferedMetrics.Inc()
	default:
		r.rm.Errors.WithLabelValues("buffer_full").Inc()
		r.rm.Buffer.DroppedMetrics.Inc()
	}
}

// DispatchBlocking will dispatch in to buf.
// If buf is full, the call will block
// note that in this case, numBuffered will contain size of buffer + number of waiting entries,
// and hence could be > bufSize
func (r *baseRoute) dispatchBlocking(buf chan []byte, in []byte) {
	r.rm.Buffer.BufferedMetrics.Inc()
	buf <- in
}
