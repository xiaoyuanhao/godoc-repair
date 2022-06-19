# godoc-generate

[![Go Report Card](https://goreportcard.com/badge/github.com/DimitarPetrov/godoc-generate)](https://goreportcard.com/report/github.com/DimitarPetrov/godoc-generate)

## Overview

`godoc-repair` is forked from [DimitarPetrov/godoc-generate](https://github.com/DimitarPetrov/godoc-generate)

`godoc-repair` is a simple command line tool that repairs godoc comments and generates default godoc comments on all
 **exported** `types`, `functions`, `consts` and `vars` in the specified directory and recursively for all subdirectories.

## Explain
* It is necessary to write comments well.
* Tool will only fix some types of comments.
* It is recommended to check the comments after repaired.

## Types
The following comments will be fixed, include type/func/const/var：

### missing name

before repair
```go
// camel case
type CamelCase struct {
}
```

after repair
```go
// CamelCase camel case
type CamelCase struct {
}
```

### missing description

before repair
```go
// CamelCase
type CamelCase struct {
}

//CamelCase2
type CamelCase2 struct {
}
```

after repair
```go
// CamelCase camel case
type CamelCase struct {
}

// CamelCase2 camel case
type CamelCase2 struct {
}
```

### with a colon

before repair
```go
// CamelCase: camel case
type CamelCase struct {
}

//CamelCase2: camel case
type CamelCase2 struct {
}
```

after repair
```go
// CamelCase camel case
type CamelCase struct {
}

// CamelCase2 camel case
type CamelCase2 struct {
}
```

### missing comment
The repaired godoc comments looks like this:
```go
// %s missing godoc.
```
Where %s is the name of the type/func/const/var.

> NOTE: The comment format can be overridden via the `--format` flag.

before repair
```go
type CamelCase struct {
}
```

after repair
```go
// CamelCase missing godoc
type CamelCase struct {
}
```

`go-repair` will add automatically comment description if enable auto description.
before repair
```go
type CamelCase struct {
}
```

after repair
```go
// CamelCase camel case
type CamelCase struct {
}
```

## Installation

#### Installing from Source
```
go install github.com/xiaoyuanhao/godoc-repair@latest
```

#### Usage
After installed, use the command to repair.
```
go-repair --code-path /path/to/your/code
```
support flag:
* --format, overwrite the default comment format.
* --code-path, code path needs to be repaired, default is the current working directory.
* --auto-description, set comment description with function name.
