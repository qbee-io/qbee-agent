package log

type Writer struct {
	level  int
	prefix string
}

func (w *Writer) Write(p []byte) (n int, err error) {
	if level < w.level {
		return len(p), nil
	}

	logf(w.level, "%s%s", w.prefix, p)

	return len(p), nil
}

// NewWriter returns new log writer with specified level and prefix.
func NewWriter(level int, prefix string) *Writer {
	return &Writer{
		level:  level,
		prefix: prefix,
	}
}
