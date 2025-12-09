package workflow

// InputValidation defines validation rules for an input parameter.
type InputValidation struct {
	Pattern       string   // regex pattern for strings
	Enum          []string // allowed values
	Min           *int     // minimum for integers
	Max           *int     // maximum for integers
	FileExists    bool     // file must exist
	FileExtension []string // allowed file extensions
}
