package metrics

type RecorderID string

type Recorder interface {
	ID() RecorderID
}
