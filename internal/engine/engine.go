package engine

import (
	"fmt"
	"log/slog"
	"strings"

	"gitlab.com/Vitrifi/tag/internal/formats"

	"gitlab.com/Vitrifi/tag/internal/chalk"
	"gitlab.com/Vitrifi/tag/internal/parser"
	"gitlab.com/Vitrifi/tag/internal/writer"
)

const (
	metaKeyValueDelimiter = "="
	doubleQuote           = '"'
	singleQuote           = '\''
	emptyString           = ""
)

func New(dryRun bool, dirPath string, sharedPath string, fileSuffix string) (Core, error) {
	if dryRun {
		slog.Info(chalk.Cyan("DRYRUN MODE"))
	}
	tmpl, err := parser.New(dirPath, sharedPath, fileSuffix)
	if err != nil {
		return Core{}, err
	}
	return Core{
		parser: tmpl,
		fwr:    writer.New(dryRun),
	}, nil
}

// Generate - generates code from
func (c *Core) Generate(data Data) error {
	parserConfig := parser.TemplateData{
		Name: data.Name,
		ParseData: parser.ParseData{
			Args: data.Args,
			Meta: generateMeta(data.MetaArgs),
			N: parser.NameOptions{
				PascalCase: formats.CasePascal(data.Name),
				CamelCase:  formats.CaseCamel(data.Name),
				SnakeCase:  formats.CaseSnake(data.Name),
				KebabCase:  formats.CaseKebab(data.Name),
				LowerCase:  strings.ToLower(data.Name),
				UpperCase:  strings.ToUpper(data.Name),
			},
		},
	}
	parsedOutput, err := c.parser.Parse(parserConfig)
	if err != nil {
		slog.Error("cannot parse input data", "error", err)
		return err
	}

	for _, item := range parsedOutput {
		var action string
		switch item.ParseData.Action {
		case parser.ActionAppend:
			if err := c.fwr.AppendFile(item.To, item.Output); err != nil {
				slog.Error("cannot appending to file", "file", item.To, "error", err)
				return err
			}
			action = chalk.Yellow("modified")
		case parser.ActionInject:
			if err := c.fwr.InjectIntoFile(item.To, item.Output, writer.Inject{
				Matcher: item.InjectMatcher,
				Clause:  writer.InjectClause(item.InjectClause),
			}); err != nil {
				slog.Error("cannot inject to file", "file", item.To, "error", err)
				return err
			}
			action = chalk.Yellow("modified")
		default:
			if err := c.fwr.WriteFile(item.To, item.Output, 0o750); err != nil {
				slog.Error("cannot writing to file", "file", item.To, "error", err)
				return err
			}
			action = chalk.Blue("created")
			if item.Notes != "" {
				message := fmt.Sprintf("%s\n%s: %s", chalk.Red("IMPORTANT"), chalk.Yellow(item.To), chalk.Green(item.Notes))
				fmt.Println(message)
			}
		}
		slog.Info(action, "file", item.To)
	}

	return nil
}

func generateMeta(meta []string) (result map[string]string) {
	result = make(map[string]string)

	if len(meta) == 0 {
		return result
	}

	for _, part := range meta {
		key, value, ok := parseKeyValue(part)
		if !ok {
			return make(map[string]string)
		}
		result[key] = value
	}

	return
}

func processValue(value string) string {
	value = strings.TrimSpace(value)
	if value == emptyString {
		return emptyString
	}

	hasMatchingQuotes := func(s string, quote rune) bool {
		return len(s) >= 2 && rune(s[0]) == quote && rune(s[len(s)-1]) == quote
	}

	if hasMatchingQuotes(value, doubleQuote) {
		quoteCount := strings.Count(value, string(doubleQuote))
		if quoteCount >= 2 {
			return value[1 : len(value)-1]
		}
	} else if value[0] == doubleQuote {
		return value[1:]
	}

	if hasMatchingQuotes(value, singleQuote) {
		quoteCount := strings.Count(value, string(singleQuote))
		if quoteCount >= 2 {
			return value[1 : len(value)-1]
		}
	} else if value[0] == singleQuote {
		return value[1:]
	}

	return value
}

func parseKeyValue(part string) (string, string, bool) {
	part = strings.TrimSpace(part)
	if part == emptyString {
		return emptyString, emptyString, false
	}

	kv := strings.SplitN(part, metaKeyValueDelimiter, 2)
	if len(kv) != 2 {
		return emptyString, emptyString, false
	}

	key := strings.TrimSpace(kv[0])
	value := processValue(kv[1])

	if key == emptyString || (value == emptyString && kv[1] == emptyString) {
		return emptyString, emptyString, false
	}

	return key, value, true
}
