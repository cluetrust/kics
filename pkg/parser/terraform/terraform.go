package terraform

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/Checkmarx/kics/pkg/model"
	"github.com/Checkmarx/kics/pkg/parser/terraform/comment"
	"github.com/Checkmarx/kics/pkg/parser/terraform/converter"
	"github.com/Checkmarx/kics/pkg/parser/utils"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// RetriesDefaultValue is default number of times a parser will retry to execute
const RetriesDefaultValue = 50

// Converter returns content json, error line, error
type Converter func(file *hcl.File, inputVariables converter.VariableMap) (model.Document, error)

// Parser struct that contains the function to parse file and the number of retries if something goes wrong
type Parser struct {
	convertFunc  Converter
	numOfRetries int
}

// NewDefault initializes a parser with Parser default values
func NewDefault() *Parser {
	return &Parser{
		numOfRetries: RetriesDefaultValue,
		convertFunc:  converter.DefaultConverted,
	}
}

// Resolve - replace or modifies in-memory content before parsing
func (p *Parser) Resolve(fileContent []byte, filename string) (*[]byte, error) {
	getInputVariables(filepath.Dir(filename))
	getDataSourcePolicy(filepath.Dir(filename))
	return &fileContent, nil
}

func processContent(elements model.Document, content, path string) {
	var certInfo map[string]interface{}
	if content != "" {
		certInfo = utils.AddCertificateInfo(path, content)
		if certInfo != nil {
			elements["certificate_body"] = certInfo
		}
	}
}

func processElements(elements model.Document, path string) {
	for k, v3 := range elements { // resource elements
		if k != "certificate_body" {
			continue
		}
		content := utils.CheckCertificate(v3.(string))
		processContent(elements, content, path)
	}
}

func processResources(doc model.Document, path string) error {
	var resourcesElements model.Document
	for _, resources := range doc { // iterate over resources
		resourcesElements = resources.(model.Document)
		for _, v2 := range resourcesElements { // resource name
			switch t := v2.(type) {
			case []interface{}:
				return errors.New("failed to process resources")
			case interface{}:
				if elements, ok := t.(model.Document); ok {
					processElements(elements, path)
				}
			}
		}
	}
	return nil
}

func addExtraInfo(json []model.Document, path string) ([]model.Document, error) {
	for _, documents := range json { // iterate over documents
		if resources, ok := documents["resource"].(model.Document); ok {
			err := processResources(resources, path)
			if err != nil {
				return []model.Document{}, err
			}
		}
	}

	return json, nil
}

func parseFile(filename string, shouldReplaceDataSource bool) (*hcl.File, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if shouldReplaceDataSource {
		replaceDataIdentifiers := regexp.MustCompile(`(data\.[A-Za-z0-9._-]+)`)
		file = []byte(replaceDataIdentifiers.ReplaceAllString(string(file), "\"$1\""))
	}
	parsedFile, _ := hclsyntax.ParseConfig(file, filename, hcl.Pos{Line: 1, Column: 1})

	return parsedFile, nil
}

// Parse execute parser for the content in a file
func (p *Parser) Parse(path string, content []byte) ([]model.Document, []int, error) {
	file, diagnostics := hclsyntax.ParseConfig(content, filepath.Base(path), hcl.Pos{Byte: 0, Line: 1, Column: 1})
	if diagnostics != nil && diagnostics.HasErrors() && len(diagnostics.Errs()) > 0 {
		err := diagnostics.Errs()[0]
		return nil, []int{}, err
	}

	ignore, err := comment.ParseComments(content, path)
	if err != nil {
		log.Err(err).Msg("failed to parse comments")
	}

	linesToIgnore := comment.GetIgnoreLines(ignore, file.Body.(*hclsyntax.Body))

	fc, parseErr := p.convertFunc(file, inputVariableMap)
	json, err := addExtraInfo([]model.Document{fc}, path)
	if err != nil {
		return json, []int{}, errors.Wrap(err, "failed terraform parse")
	}

	return json, linesToIgnore, errors.Wrap(parseErr, "failed terraform parse")
}

// SupportedExtensions returns Terraform extensions
func (p *Parser) SupportedExtensions() []string {
	return []string{".tf", ".tfvars"}
}

// SupportedTypes returns types supported by this parser, which are terraform
func (p *Parser) SupportedTypes() []string {
	return []string{"Terraform"}
}

// GetKind returns Terraform kind parser
func (p *Parser) GetKind() model.FileKind {
	return model.KindTerraform
}

// GetCommentToken return the comment token of Terraform - #
func (p *Parser) GetCommentToken() string {
	return "#"
}

// StringifyContent converts original content into string formated version
func (p *Parser) StringifyContent(content []byte) (string, error) {
	return string(content), nil
}
