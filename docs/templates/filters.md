# Filter Reference

TAG provides custom filters for common code generation tasks, in addition to Gonja's built-in filters.

## Usage

Apply filters using the pipe (`|`) operator:

```jinja2
{{ vars.project_name|snake }}
{{ vars.name|plural|pascal }}
```

Filters can be chained (applied left to right).

## Case Transformation Filters

| Filter | Input | Output | Description |
|--------|-------|--------|-------------|
| `snake` | `MyProject` | `my_project` | Convert to snake_case |
| `pascal` | `my_project` | `MyProject` | Convert to PascalCase |
| `camel` | `my_project` | `myProject` | Convert to camelCase |
| `kebab` | `MyProject` | `my-project` | Convert to kebab-case |
| `lower` | `MyProject` | `myproject` | Convert to lowercase |
| `upper` | `MyProject` | `MYPROJECT` | Convert to UPPERCASE |
| `title` | `my project` | `My Project` | Convert to Title Case |

### Aliases

For convenience, these aliases are also available:

| Alias | Equivalent |
|-------|------------|
| `snake_case` | `snake` |
| `pascal_case` | `pascal` |
| `camel_case` | `camel` |
| `kebab_case` | `kebab` |

### Examples

```jinja2
{{ "MyAwesomeProject"|snake }}     {# my_awesome_project #}
{{ "user_service"|pascal }}         {# UserService #}
{{ "UserHandler"|camel }}           {# userHandler #}
{{ "UserService"|kebab }}           {# user-service #}
```

## Inflection Filters

| Filter | Input | Output | Description |
|--------|-------|--------|-------------|
| `plural` | `user` | `users` | Pluralize word |
| `singular` | `users` | `user` | Singularize word |
| `past` | `OrderCancel` | `OrderCancelled` | Convert last word to past tense |
| `ordinalize` | `3` | `3rd` | Add ordinal suffix |
| `titleize` | `my_project` | `My Project` | Convert to readable title |
| `humanize` | `my_project` | `My project` | Convert to sentence form |

### Aliases

| Alias | Equivalent |
|-------|------------|
| `pluralize` | `plural` |
| `singularize` | `singular` |
| `past_tense` | `past` |

### Examples

```jinja2
{{ "user"|plural }}             {# users #}
{{ "categories"|singular }}     {# category #}
{{ "1"|ordinalize }}            {# 1st #}
{{ "2"|ordinalize }}            {# 2nd #}
{{ "user_profile"|titleize }}   {# User Profile #}
{{ "user_profile"|humanize }}   {# User profile #}
{{ "OrderCancel"|past }}        {# OrderCancelled #}
{{ "order_create"|past }}       {# order_created #}
{{ "orderRun"|past }}           {# orderRan #}
```

The `past` filter converts the **last word** to past tense, preserving the original casing style (PascalCase, camelCase, snake_case, kebab-case). It handles:

- **Regular verbs**: add `-ed` rules (create → created, process → processed)
- **Irregular verbs**: ~50 common software-domain verbs (send → sent, build → built, run → ran)
- **Consonant doubling**: whitelist-based (cancel → cancelled, commit → committed, stop → stopped)
- **Special endings**: `-ic` → `-icked` (panic → panicked), consonant+`y` → `-ied` (copy → copied)

## String Operation Filters

### split

Split a string by delimiter.

**Syntax:** `split` or `split:"delimiter"`

```jinja2
{{ "a,b,c"|split:"," }}                {# ["a", "b", "c"] #}
{{ "hello world"|split }}              {# ["hello", "world"] (whitespace) #}
{{ "one-two-three"|split:"-" }}        {# ["one", "two", "three"] #}
```

### join

Join a list with a separator.

**Syntax:** `join:"separator"`

```jinja2
{{ ["a", "b", "c"]|join:", " }}        {# a, b, c #}
{{ vars.features|join:" | " }}         {# feat1 | feat2 | feat3 #}
```

### contains

Check if a string contains a substring.

**Syntax:** `contains:"substring"`

```jinja2
{% if vars.name|contains:"admin" %}
  Has admin in name
{% endif %}
```

### hasprefix

Check if a string starts with a prefix.

**Syntax:** `hasprefix:"prefix"`

```jinja2
{% if vars.module|hasprefix:"github.com" %}
  GitHub module
{% endif %}
```

### hassuffix

Check if a string ends with a suffix.

**Syntax:** `hassuffix:"suffix"`

```jinja2
{% if vars.filename|hassuffix:".go" %}
  Go file
{% endif %}
```

### replace

Replace all occurrences of a substring.

**Syntax:** `replace:"old":"new"`

```jinja2
{{ "hello world"|replace:"world":"universe" }}   {# hello universe #}
{{ vars.path|replace:"/":"_" }}                  {# convert slashes #}
```

### trim

Remove leading and trailing whitespace (or specified characters).

**Syntax:** `trim` or `trim:"chars"`

```jinja2
{{ "  hello  "|trim }}                 {# hello #}
{{ "...hello..."|trim:"." }}           {# hello #}
```

### default

Provide a default value for empty/nil values.

**Syntax:** `default:"value"`

```jinja2
{{ vars.author|default:"Anonymous" }}
{{ vars.port|default:8080 }}
```

Returns the default if the input is:
- `nil` / `null`
- Empty string `""`
- Error value

### truncate

Truncate a string to a maximum length.

**Syntax:** `truncate:length` or `truncate:length:"ellipsis"`

```jinja2
{{ "Hello World"|truncate:5 }}              {# Hello... #}
{{ "Hello World"|truncate:5:"" }}           {# Hello #}
{{ "Hello World"|truncate:5:"…" }}          {# Hello… #}
{{ vars.description|truncate:100 }}
```

The default ellipsis is `...`.

## Dialect Type-Mapping Filter

The `to` filter maps canonical type names to language-specific types using the dialect system. This enables a single template to target multiple languages or standards by switching the dialect name.

### to

**Syntax:** `to("dialect_name")`

```jinja
{{ "uuid" | to("go") }}         {# string #}
{{ "uuid" | to("postgres") }}   {# UUID #}
{{ "datetime" | to("go") }}     {# time.Time #}
{{ "int64" | to("mysql") }}     {# BIGINT #}
{{ "bool" | to("typescript") }} {# boolean #}
```

**Built-in dialects:** `go`, `postgres`, `mysql`, `typescript`, `openapi`, `protobuf`

**Canonical types:** `string`, `text`, `int`, `int32`, `int64`, `float`, `float32`, `float64`, `bool`, `byte`, `bytes`, `uuid`, `datetime`, `date`, `decimal`, `json`

**Error behavior:** Using an unmapped type or unknown dialect produces a template rendering error (not silent passthrough). This ensures generated code is always correct.

**Example — multi-target template:**

```jinja
{# Go struct #}
type {{ vars.model_name | pascal }} struct {
{% for field in vars.fields %}
    {{ field.name | pascal }} {{ field.type | to("go") }}
{% endfor %}
}

{# SQL table #}
CREATE TABLE {{ vars.table_name | snake }} (
{% for field in vars.fields %}
    {{ field.name | snake }} {{ field.type | to("postgres") }}{% if not loop.last %},{% endif %}
{% endfor %}
);
```

**Dialect overrides:** Place YAML files in `_dialects/` within a template to override built-in mappings. See `tag dialect list` and `tag dialect show <name>` for available dialects and their mappings.

## Built-in Gonja Filters

TAG also includes all standard Gonja/Jinja2 filters:

### String Filters

| Filter | Description |
|--------|-------------|
| `capitalize` | Capitalize first character |
| `center:width` | Center in field of given width |
| `escape` or `e` | HTML escape |
| `format:fmt` | String formatting (Go fmt syntax) |
| `indent:width` | Indent each line |
| `ljust:width` | Left-justify in field |
| `rjust:width` | Right-justify in field |
| `safe` | Mark as safe (no escaping) |
| `striptags` | Remove HTML tags |
| `wordwrap:width` | Wrap at word boundaries |

### List Filters

| Filter | Description |
|--------|-------------|
| `batch:size` | Split into batches |
| `first` | Get first element |
| `last` | Get last element |
| `length` | Get length |
| `list` | Convert to list |
| `random` | Get random element |
| `reverse` | Reverse order |
| `shuffle` | Randomly shuffle |
| `slice:start:stop` | Get slice |
| `sort` | Sort elements |
| `unique` | Remove duplicates |

### Numeric Filters

| Filter | Description |
|--------|-------------|
| `abs` | Absolute value |
| `float` | Convert to float |
| `int` | Convert to integer |
| `round` | Round to nearest integer |
| `filesizeformat` | Format as file size |

### Type Filters

| Filter | Description |
|--------|-------------|
| `string` | Convert to string |
| `tojson` | Convert to JSON |

## Filter Combinations

Filters can be chained for powerful transformations:

```jinja2
{# model name variations #}
{{ vars.model|plural|pascal }}         {# Users #}
{{ vars.model|plural|snake }}          {# users #}
{{ vars.model|singular|pascal }}       {# User #}

{# file names #}
{{ vars.service|snake }}_service.go    {# user_service.go #}
{{ vars.handler|kebab }}-handler.ts    {# user-handler.ts #}

{# event name variations #}
{{ vars.event|past|pascal }}          {# OrderCancelled #}
{{ vars.event|past|snake }}           {# order_cancelled #}

{# clean and transform #}
{{ vars.name|trim|snake|lower }}
```

## Common Patterns

### Go Type Names

```jinja2
type {{ vars.model|pascal }}Service struct {}
func New{{ vars.model|pascal }}Service() *{{ vars.model|pascal }}Service {}
```

### Package Names

```jinja2
package {{ vars.module|snake }}
```

### File Names

```jinja2
{# In path: __model | snake__/service.go.tmpl #}
```

### URL Slugs

```jinja2
{{ vars.title|kebab }}
```

### Event Names

```jinja2
type {{ vars.event|past }}Event struct {}    {# OrderCancelledEvent #}
{{ vars.event|past|snake }}_event.go         {# order_cancelled_event.go #}
```

### Database Tables

```jinja2
{{ vars.model|plural|snake }}
```

### Environment Variables

```jinja2
{{ vars.name|upper|replace:" ":"_" }}
```

## Error Handling

If a filter encounters an error:
- The error is propagated through the filter chain
- The template execution continues but outputs the error
- Check your variable types match the filter's expected input

Example error cases:
- `split` on a non-string value
- `truncate` with a negative length
- `replace` with wrong number of arguments

## See Also

- [Template Syntax](syntax.md) - Complete syntax guide
- [Template Authoring](authoring.md) - Creating templates
- [Gonja Documentation](https://github.com/noirbizarre/gonja) - Full Gonja reference
