# TAG

This is a code generator based on [Pyrotic](https://github.com/code-gorilla-au/pyrotic) but a few perks and features added.

## Motivation

Pyrotic is cool. I like it. But, it is incomplete and insufficient for the uses I need.  
So, I took it upon myself to modify it and improve it to have a much better template generation system.

## Install

```
go install github.com/kaikenlabs/tag@latest
```

## Initialise the templates directory

The initial setup creates a `_templates` directory at the root of the project to hold the generators:

```
tag init
```

## Work with generators

Now you can create your first generator:

```
tag new my_generator
```

A generator is a template or a collection of templates that can be created or added to existing files.  
To edit your generator, go to the `_templates` directory created and edit it.  
_TAG_ uses [Go's template language](https://pkg.go.dev/text/template#pkg-overview) so you can build your templates with
ease.

Go [here](#data-exposed-to-each-template) to understand what data is exposed to each template.

## Run your generator

Default template path is `_templates` and default file extension is `.tmpl`

```sh
tag generate <name of generator> <name-to-pass-in> <args>
```

As in our example:

```sh
tag generate my_generator myObject
```

## Hooks

`tag` offers the ability of adding pre- and post- hooks for both generators and bundles.

As an example, for Go it could be useful to run:

```json
{
  "env":{
    "TAG_PATH": "_templates",
    "TAG_EXTENSION": ".tmpl",
    "TAG_SHARED_PATH": "_shared",
    "TAG_BUNDLE_PATH": "_bundles"
  },
  "hooks": {
    "pre": [],
    "post":[
      ["./bin/goimports","-w", "-l","."],
      ["./bin/gofumpt", "-w", "-l", "."],
      ["./bin/golangci-lint", "run"]
    ]
  }
}
```

**IMPORTANT**: all the hooks will run from the directory that `tag` is being called in, so make sure you add the relative path.

## Tips & Tricks

### Data exposed to each template

_TAG_ will expose the following data to each template:

| Property | Description                                                                                            | Type                | Default | Example                           |
|----------|--------------------------------------------------------------------------------------------------------|---------------------|---------|-----------------------------------|
| Name     | The name you passed in the command line                                                                | `string`            | ""      | `my_generator`                    |
| Args     | The arguments you passed in the command line. It is free form so you can manipulate it in the template | `string`            | nil     | `age:int,name:string`             |
| Meta     | Additional arguments you can pass from the command line with the `--meta` flag                         | `map[string]string` | nil     | `map[string]string{"foo": "bar"}` |

A quick example:

`tag -d generante myGenerator myName name:string,age:int`

with the following template:

```
My generator with name {{ .Name }}.
I can use now the args:

{{ $argList := splitByDelimiter .Args "," }}

{{- range $i, $item := $argList }}
    {{- $fieldDef := splitByDelimiter $item ":" }}
    {{ index $fieldDef 0 | casePascal }} {{ index $fieldDef 1 }} `json:"{{ $name_lower }}"`
{{- end }}
```

Will result in:

```
My generator with name myGenerator.
I can use now the args:

Name string `json:"name"`
Age int `json:"age"`
```

In addition to this, _TAG_ provides a special object `N`.\
This object facilitates getting the `name` you pass in the command line in different formats:

* PascalCase // `MyGenerator`
* CamelCase  // `myGenerator`
* SnakeCase  // `my_generator`
* KebabCase  // `my-generator`
* LowerCase  // `mygenerator`
* UpperCase  // `MYGENERATOR`

So, in your template you can use. `{{ .N.PascalCase }}` to make things easier. :)

### Use different directory

```
tag --path example/_templates generate cmd --name setup
tag -p example/_templates generate cmd --name setup
```

### Use different file extension

The default file extension is `.tmpl`

```
tag --extension ".template" generate cmd --name setup
tag -x ".template" generate cmd --name setup
```

### Dry run mode

Dry run will log to console rather than write to file

```
tag -d generate cmd --name setup
tag --dry-run generate cmd --name setup
```

### Different shared folder

Default shared templates path is `_templates/shared`

```
tag --shared foo/bar generate cmd --name setup
tag -s foo/bar generate cmd --name setup
```

### Dev mode

provides the short file name with logging

```bash
ENV=DEV tag -p example/_templates generate fakr --meta foo=bar,bin=baz
```

### Using shared templates

In some instances you will want to reuse some templates across multiple generators. This can be done by having a
`shared` directory within the `_templates` directory.\
Any templates that are declared in the [shared](example/_templates/shared/config.tmpl) directory will be loading along
with the generator.\
[Reference](example/_templates/fakr/shared_config.tmpl) the shared template within your generator directory in order to
inject / append / create file.

## Bundles

A bundle is a collection of templates you can run all in one go.

### Creating bundles

You can create a bundle like this:

`tag new-bundle myBundle`

By default, bundles are located in the directory `_bundles` inside of the templates directory.\

If you open the generate file, you can see is a simple `JSON` object in which you can add generators.

### Running bundles

To run a bundle you can use the following command:

`tag generate -b myBundle myName myArgs`

See that the command is very similar to the one we use to run a single generator, but with the addition of the `-b` flag.

This command will run all the generators inside of the bundle file.

## Formatter properties

Formatter will pick up any of these variables within the `---` block and hydrate the metadata for the template.\
Any properties matching the signature will be added to the Meta property, for example `foo: bar` will be accessible by
`{{ Meta.foo }}`. View more [examples](example/_templates).

| Property | Type          | Default | Example                 |
|----------|---------------|---------|-------------------------|
| to:      | string (path) | ""      | src/lib/utils/readme.md |
| append:  | bool          | false   | false                   |
| inject:  | bool          | false   | false                   |
| before:  | string        | ""      | type config struct      |
| after:   | string        | ""      | // commands             |

## Built-in template functions and inflectors

ships with some already built in template functions, some [examples](example/_templates/fakr/farkr_case.tmpl)

| func name           | description                         | code example                                | result                        |
|---------------------|-------------------------------------|---------------------------------------------|-------------------------------| 
| caseSnake           | convert to snake case               | {{ MetaData \| caseSnake }}                 | meta_data                     |
| caseKebab           | convert to kebab case               | {{ MetaData \| caseKebab }}                 | meta-data                     |
| casePascal          | convert to pascal case              | {{ meta_data \| casePascal }}               | MetaData                      |
| caseLower           | convert to lower case               | {{ MetaData \| caseLower }}                 | metadata                      |
| caseTitle           | convert to title case               | {{ MetaData \| caseTitle }}                 | METADATA                      |
| caseCamel           | convert to camel case               | {{ MetaData \| caseCamel }}                 | metaData                      |
| splitByDelimiter    | splits string by delimiter          | {{ splitByDelimiter "long,list" "," }}      | []string{"long" "list"}       |
| splitAfterDelimiter | splits string after delimiter       | {{ splitAfterDelimiter "a,long,list" "," }} | []string{"a," "long," "list"} |
| contains            | checks if string contains substring | {{ contains "foobarbin" "bar" }}            | true                          |
| hasPrefix           | checks if string has the prefix     | {{ contains "foobarbin" "foo" }}            | true                          |
| hasSuffix           | checks if string has the suffix     | {{ contains "foobarbin" "bin" }}            | true                          |

`tag` also provide some Inflections using [flect](https://github.com/gobuffalo/flect)

- pluralise
- singularise
- ordinalize
- titleize
- humanize

## Pass in meta via the command line

you can pass in meta data via the `--meta` or `-m` flag, which takes in a comma (`,`) delimited list, and overrides the
`{{ .Meta.<your-property> }}` within the template.

```
tag generate fakr --meta foo=bar,bin=baz
tag generate fakr -m foo=bar,bin=baz
```

## Sample output

```sh
$ bin/tag generate -b scaffold UserSetting "settings:types.MessagePack"
[11:55:01.772] tag info: running bundle bundle="scaffold" target="UserSetting"
[11:55:01.775] tag info: created file="internal/adapters/user_setting.go"
[11:55:01.776] tag info: created file="internal/models/user_setting.go"
[11:55:01.778] tag info: created file="internal/controllers/user_setting_controller.go"
[11:55:01.780] tag info: created file="internal/services/user_setting_service.go"
[11:55:01.781] tag info: modified file="internal/persistence/repository.go"
[11:55:01.782] tag info: modified file="internal/persistence/interfaces.go"
[11:55:01.784] tag info: created file="internal/persistence/postgres/user_setting_repository.go"
[11:55:01.791] tag info: modified file="internal/persistence/postgres/repository.go"
[11:55:01.797] tag info: modified file="internal/persistence/postgres/repository.go"
[11:55:01.798] tag info: modified file="internal/persistence/postgres/helpers.go"
[11:55:01.799] tag info: created file="tests/component/user_setting_stage_test.go"
[11:55:01.799] tag info: created file="tests/component/user_setting_test.go"
[11:55:01.800] tag info: modified file="tests/component/common.go"
[11:55:01.800] tag info: modified file="internal/server/router.go"
[11:55:01.808] tag info: modified file="internal/server/router.go"
[11:55:01.808] tag info: modified file="db/migrate.go"
[11:55:01.808] tag info: running hook hook="./bin/goimports -w -l ."
db/migrate.go
internal/adapters/user_setting.go
internal/controllers/user_setting_controller.go
internal/models/user_setting.go
internal/persistence/interfaces.go
internal/persistence/postgres/helpers.go
internal/persistence/postgres/repository.go
internal/persistence/postgres/user_setting_repository.go
internal/persistence/repository.go
internal/server/router.go
internal/services/user_setting_service.go
tests/component/common.go
tests/component/user_setting_stage_test.go
tests/component/user_setting_test.go

[11:55:04.012] tag info: running hook hook="./bin/gofumpt -w -l ."
internal/adapters/user_setting.go
tests/component/user_setting_stage_test.go

[11:55:04.292] tag info: running hook hook="./bin/golangci-lint run"
```