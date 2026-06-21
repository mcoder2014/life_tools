package main

type tailWriter struct {
	limit int
	buf   []byte
}

func newTailWriter(limit int) *tailWriter {
	return &tailWriter{limit: limit}
}

func (w *tailWriter) Write(p []byte) (int, error) {
	if w.limit <= 0 {
		return len(p), nil
	}

	if len(p) >= w.limit {
		w.buf = append(w.buf[:0], p[len(p)-w.limit:]...)
		return len(p), nil
	}

	w.buf = append(w.buf, p...)
	if len(w.buf) > w.limit {
		copy(w.buf, w.buf[len(w.buf)-w.limit:])
		w.buf = w.buf[:w.limit]
	}

	return len(p), nil
}

func (w *tailWriter) String() string {
	return string(w.buf)
}
