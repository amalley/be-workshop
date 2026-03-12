package models

// Note: Models copied from https://github.com/LauchlanT/wikistats/blob/main/ch-1/pkg/models/message.go (thanks :) )

type WikiStreamMessage struct {
	Schema           string                     `json:"$schema"`
	Meta             WikiStreamMessageMeta      `json:"meta"`
	ID               int64                      `json:"id"`
	Type             string                     `json:"type"`
	Namespace        int                        `json:"namespace"`
	Title            string                     `json:"title"`
	TitleURL         string                     `json:"title_url"`
	Comment          string                     `json:"comment"`
	Timestamp        int64                      `json:"timestamp"`
	User             string                     `json:"user"`
	Bot              bool                       `json:"bot"`
	NotifyURL        string                     `json:"notify_url"`
	ServerURL        string                     `json:"server_url"`
	ServerName       string                     `json:"server_name"`
	ServerScriptPath string                     `json:"server_script_path"`
	Wiki             string                     `json:"wiki"`
	ParsedComment    string                     `json:"parsedcomment"`
	Minor            *bool                      `json:"minor,omitempty"`
	Length           *WikiStreamMessageLength   `json:"length,omitempty"`
	Revision         *WikiStreamMessageRevision `json:"revision,omitempty"`
}

type WikiStreamMessageMeta struct {
	URI       string `json:"uri"`
	RequestID string `json:"request_id"`
	ID        string `json:"id"`
	DT        string `json:"dt"`
	Domain    string `json:"domain"`
	Stream    string `json:"stream"`
	Topic     string `json:"topic"`
	Partition int    `json:"partition"`
	Offset    int64  `json:"offset"`
}

type WikiStreamMessageLength struct {
	Old int `json:"old"`
	New int `json:"new"`
}

type WikiStreamMessageRevision struct {
	Old int `json:"old"`
	New int `json:"new"`
}
