package export

type ExportFormat int

const (
	FormatMarkdown ExportFormat = iota
	FormatPlainText
	FormatJSON
)

type ExportOptions struct {
	IncludeSystemMessages bool
	IncludeReasoning      bool
	IncludeToolCalls      bool
	IncludeMetadata       bool
	Format                ExportFormat
	TruncateResults       bool
	MaxResultLines        int
}

type ExportResult struct {
	Content  string
	Filepath string
	Size     int64
	Messages int
}

var DefaultExportOptions = ExportOptions{
	IncludeSystemMessages: false,
	IncludeReasoning:      true,
	IncludeToolCalls:      true,
	IncludeMetadata:       true,
	Format:                FormatMarkdown,
	TruncateResults:       true,
	MaxResultLines:        500,
}
