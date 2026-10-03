package shortcut

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/samber/lo"

	"github.com/wuhan005/sayrud/internal/ai/openai"
	"github.com/wuhan005/sayrud/internal/db"
)

// builtin is a built-in AI shortcut, its labels are message keys.
type builtin struct {
	// id is the shortcut ID the fields refer to, e.g. "ai_classify".
	id string
	// name is the message key of the name.
	name string
	// description is the message key of the description.
	description string
	// resultTypes are the types of the fields the shortcut can be attached to.
	resultTypes []db.SLFieldType
	// formItems are the inputs, their labels, placeholders and option labels are message keys.
	formItems []db.ShortcutFormItem
	// prompt builds the messages from the inputs, it returns *Error if the inputs can not be used.
	prompt func(in promptInput) ([]openai.Message, error)
}

// promptInput is the resolved inputs of a built-in shortcut.
type promptInput struct {
	// field is the field being generated, e.g. its options are the categories to choose from.
	field *db.SLField
	// texts are the texts of the inputs keyed by the form item key.
	texts map[string]string
}

func (b *builtin) definition() *Definition {
	return &Definition{
		ID:          b.id,
		Kind:        KindAI,
		Name:        b.name,
		Description: b.description,
		ResultTypes: b.resultTypes,
		FormItems:   b.formItems,
		builtin:     b,
	}
}

var sourceItem = db.ShortcutFormItem{
	Key:       "source",
	Label:     "shortcut::form_source",
	Component: db.ShortcutFormFieldSelect,
	Required:  true,
}

func requirementItem(placeholder string) db.ShortcutFormItem {
	return db.ShortcutFormItem{
		Key:         "requirement",
		Label:       "shortcut::form_requirement",
		Component:   db.ShortcutFormTextarea,
		Placeholder: placeholder,
	}
}

// translateLanguages are the target languages of the translation, the values are the English names given to the model.
var translateLanguages = []db.ShortcutFormOption{
	{Value: "English", Label: "shortcut::lang_en"},
	{Value: "Simplified Chinese", Label: "shortcut::lang_zh_cn"},
	{Value: "Traditional Chinese", Label: "shortcut::lang_zh_tw"},
	{Value: "Japanese", Label: "shortcut::lang_ja"},
	{Value: "Korean", Label: "shortcut::lang_ko"},
	{Value: "Spanish", Label: "shortcut::lang_es"},
	{Value: "Portuguese", Label: "shortcut::lang_pt"},
	{Value: "French", Label: "shortcut::lang_fr"},
	{Value: "German", Label: "shortcut::lang_de"},
	{Value: "Russian", Label: "shortcut::lang_ru"},
}

const contentFence = "<<<\n%s\n>>>"

func content(text string) string {
	return strings.ReplaceAll(contentFence, "%s", text)
}

func withRequirement(text, requirement string) string {
	if strings.TrimSpace(requirement) == "" {
		return text
	}
	return text + "\n\nAdditional requirements:\n" + requirement
}

func bulletList(items []string) string {
	return "- " + strings.Join(items, "\n- ")
}

func requireOptions(field *db.SLField) ([]string, error) {
	names := lo.Filter(optionNames(field), func(name string, _ int) bool { return strings.TrimSpace(name) != "" })
	if len(names) == 0 {
		return nil, newError("shortcut::options_required")
	}
	return names, nil
}

var builtinOrder = []string{"ai_classify", "ai_tag", "ai_translate", "ai_summarize", "ai_extract", "ai_custom"}

var builtins = map[string]*builtin{
	"ai_classify": {
		id:          "ai_classify",
		name:        "shortcut::ai_classify",
		description: "shortcut::ai_classify_description",
		resultTypes: []db.SLFieldType{db.SingleSelectFieldType},
		formItems:   []db.ShortcutFormItem{sourceItem, requirementItem("shortcut::ai_classify_placeholder")},
		prompt: func(in promptInput) ([]openai.Message, error) {
			names, err := requireOptions(in.field)
			if err != nil {
				return nil, err
			}
			return []openai.Message{
				{Role: "system", Content: withRequirement("You are a data classification assistant. Classify the content into exactly one of the categories. "+
					"Reply with the category name only, exactly as written in the list, without explanation or punctuation.", in.texts["requirement"])},
				{Role: "user", Content: "Categories:\n" + bulletList(names) + "\n\nContent:\n" + content(in.texts["source"])},
			}, nil
		},
	},
	"ai_tag": {
		id:          "ai_tag",
		name:        "shortcut::ai_tag",
		description: "shortcut::ai_tag_description",
		resultTypes: []db.SLFieldType{db.MultiSelectFieldType},
		formItems:   []db.ShortcutFormItem{sourceItem, requirementItem("shortcut::ai_tag_placeholder")},
		prompt: func(in promptInput) ([]openai.Message, error) {
			names, err := requireOptions(in.field)
			if err != nil {
				return nil, err
			}
			return []openai.Message{
				{Role: "system", Content: withRequirement("You are a data tagging assistant. Choose all the tags that apply to the content. "+
					`Reply with a JSON array of the tag names exactly as written in the list, e.g. ["A","B"], or [] if none applies, without explanation.`, in.texts["requirement"])},
				{Role: "user", Content: "Tags:\n" + bulletList(names) + "\n\nContent:\n" + content(in.texts["source"])},
			}, nil
		},
	},
	"ai_translate": {
		id:          "ai_translate",
		name:        "shortcut::ai_translate",
		description: "shortcut::ai_translate_description",
		resultTypes: []db.SLFieldType{db.TextFieldType},
		formItems: []db.ShortcutFormItem{
			sourceItem,
			{
				Key:       "language",
				Label:     "shortcut::form_language",
				Component: db.ShortcutFormSelect,
				Required:  true,
				Options:   translateLanguages,
				Default:   "English",
			},
		},
		prompt: func(in promptInput) ([]openai.Message, error) {
			return []openai.Message{
				{Role: "system", Content: "You are a professional translator. Translate the content into " + in.texts["language"] +
					". Keep the meaning, tone and formatting. Reply with the translation only, without explanation."},
				{Role: "user", Content: content(in.texts["source"])},
			}, nil
		},
	},
	"ai_summarize": {
		id:          "ai_summarize",
		name:        "shortcut::ai_summarize",
		description: "shortcut::ai_summarize_description",
		resultTypes: []db.SLFieldType{db.TextFieldType},
		formItems:   []db.ShortcutFormItem{sourceItem, requirementItem("shortcut::ai_summarize_placeholder")},
		prompt: func(in promptInput) ([]openai.Message, error) {
			return []openai.Message{
				{Role: "system", Content: withRequirement("You are a summarization assistant. Summarize the content concisely in the same language as the content. "+
					"Reply with the summary only, without explanation.", in.texts["requirement"])},
				{Role: "user", Content: content(in.texts["source"])},
			}, nil
		},
	},
	"ai_extract": {
		id:          "ai_extract",
		name:        "shortcut::ai_extract",
		description: "shortcut::ai_extract_description",
		resultTypes: []db.SLFieldType{db.TextFieldType, db.NumberFieldType},
		formItems: []db.ShortcutFormItem{
			sourceItem,
			{
				Key:         "target",
				Label:       "shortcut::form_target",
				Component:   db.ShortcutFormInput,
				Required:    true,
				Placeholder: "shortcut::ai_extract_placeholder",
			},
		},
		prompt: func(in promptInput) ([]openai.Message, error) {
			format := "Reply with the extracted text only, without explanation. Reply NONE if it is not found."
			if in.field.Type == db.NumberFieldType {
				format = "Reply with the number only, in digits without units or thousands separators. Reply NONE if it is not found."
			}
			return []openai.Message{
				{Role: "system", Content: "You are an information extraction assistant. Extract the following information from the content: " +
					in.texts["target"] + ". " + format},
				{Role: "user", Content: content(in.texts["source"])},
			}, nil
		},
	},
	"ai_custom": {
		id:          "ai_custom",
		name:        "shortcut::ai_custom",
		description: "shortcut::ai_custom_description",
		resultTypes: []db.SLFieldType{db.TextFieldType, db.NumberFieldType, db.SingleSelectFieldType, db.MultiSelectFieldType},
		formItems: []db.ShortcutFormItem{
			{
				Key:         "prompt",
				Label:       "shortcut::form_prompt",
				Component:   db.ShortcutFormPrompt,
				Required:    true,
				Placeholder: "shortcut::ai_custom_placeholder",
			},
		},
		prompt: func(in promptInput) ([]openai.Message, error) {
			format := "Reply with the result only, without explanation. Reply NONE if there is no result."
			switch in.field.Type {
			case db.NumberFieldType:
				format = "Reply with a number only, in digits without units or thousands separators. Reply NONE if there is no result."
			case db.SingleSelectFieldType:
				names, err := requireOptions(in.field)
				if err != nil {
					return nil, err
				}
				format = "Reply with exactly one of the following values, as written, without explanation:\n" + bulletList(names)
			case db.MultiSelectFieldType:
				names, err := requireOptions(in.field)
				if err != nil {
					return nil, err
				}
				format = `Reply with a JSON array of the values chosen from the following list, as written, e.g. ["A","B"], without explanation:` + "\n" + bulletList(names)
			}
			return []openai.Message{
				{Role: "system", Content: "You process the data of a table record by the instruction of the user. " + format},
				{Role: "user", Content: in.texts["prompt"]},
			}, nil
		},
	},
}

var (
	codeFencePattern = regexp.MustCompile("(?s)^```[a-zA-Z]*\\s*(.*?)\\s*```$")
	jsonArrayPattern = regexp.MustCompile(`(?s)\[.*\]`)
	listSeparators   = regexp.MustCompile(`[,，、;；\n]+`)
)

// cleanCompletion strips the code fence and the quotes around the whole completion.
func cleanCompletion(text string) string {
	text = strings.TrimSpace(text)
	if m := codeFencePattern.FindStringSubmatch(text); m != nil {
		text = strings.TrimSpace(m[1])
	}
	for _, pair := range [][2]string{{`"`, `"`}, {"'", "'"}, {"“", "”"}, {"「", "」"}, {"`", "`"}} {
		if len(text) >= 2 && strings.HasPrefix(text, pair[0]) && strings.HasSuffix(text, pair[1]) {
			text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, pair[0]), pair[1]))
		}
	}
	return text
}

func isNone(text string) bool {
	return text == "" || strings.EqualFold(text, "none") || strings.EqualFold(text, "null") || strings.EqualFold(text, "n/a")
}

// parseCompletion converts the completion to the result for CellValue, being lenient to the common deviations of the models.
func parseCompletion(field *db.SLField, completion string) (interface{}, error) {
	text := cleanCompletion(completion)
	switch field.Type {
	case db.NumberFieldType:
		if isNone(text) {
			return nil, nil
		}
		return text, nil

	case db.SingleSelectFieldType:
		if isNone(text) {
			return nil, nil
		}
		text = strings.TrimRight(text, ".。!！")
		options := fieldOptions(field)
		if _, ok := matchOption(options, text); ok {
			return text, nil
		}

		// The model may wrap the category in a sentence, accept it if exactly one category is mentioned, preferring the longest name.
		var found []selectOption
		for _, o := range options {
			if o.Name != "" && strings.Contains(strings.ToLower(text), strings.ToLower(o.Name)) {
				found = append(found, o)
			}
		}
		found = dropContained(found)
		if len(found) == 1 {
			return found[0].Name, nil
		}
		return text, nil

	case db.MultiSelectFieldType:
		var names []string
		if raw := jsonArrayPattern.FindString(text); raw == "" || json.Unmarshal([]byte(raw), &names) != nil {
			names = lo.Map(listSeparators.Split(text, -1), func(s string, _ int) string { return cleanCompletion(s) })
		}

		options := fieldOptions(field)
		known := lo.Filter(names, func(name string, _ int) bool {
			_, ok := matchOption(options, name)
			return ok
		})
		candidates := lo.Filter(names, func(name string, _ int) bool { return !isNone(name) })
		if len(known) == 0 && len(candidates) > 0 {
			return nil, newError("shortcut::invalid_output").withDetail("%q", truncate(text, 50))
		}
		return lo.ToAnySlice(known), nil

	default:
		if isNone(text) {
			return nil, nil
		}
		return text, nil
	}
}

// dropContained drops the options whose name is contained in another found option, e.g. "Bug" in "UI Bug".
func dropContained(options []selectOption) []selectOption {
	return lo.Filter(options, func(o selectOption, _ int) bool {
		return !lo.ContainsBy(options, func(other selectOption) bool {
			return other.UID != o.UID && len(other.Name) > len(o.Name) && strings.Contains(strings.ToLower(other.Name), strings.ToLower(o.Name))
		})
	})
}
