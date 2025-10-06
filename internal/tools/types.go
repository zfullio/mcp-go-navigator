package tools

// ------------------ list packages ------------------

// ListPackagesInput содержит входные данные для инструмента ListPackages.
type ListPackagesInput struct {
	// Dir - корневая директория для сканирования Go-пакетов
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go packages"`
}

// ListPackagesOutput содержит результаты работы инструмента ListPackages.
type ListPackagesOutput struct {
	// Packages - список найденных Go-пакетов
	Packages []string `json:"packages" jsonschema:"List of discovered Go package import paths"`
}

// ------------------ list symbols ------------------

// ListSymbolsInput содержит входные данные для инструмента ListSymbols.
type ListSymbolsInput struct {
	// Dir - корневая директория Go-модуля
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Package - путь к пакету для поиска символов
	Package string `json:"package" jsonschema:"Package path to inspect for symbols"`
}

// Symbol представляет собой символ (функцию, структуру, интерфейс и т.д.) в Go-коде.
type Symbol struct {
	// Kind - тип символа (func, struct, interface, method и т.д.)
	Kind string `json:"kind" jsonschema:"Symbol type (func, struct, interface, method, etc.)"`
	// Name - имя символа
	Name string `json:"name" jsonschema:"Symbol name"`
	// Package - пакет, в котором определен символ
	Package string `json:"package" jsonschema:"Package where the symbol is defined"`
	// File - файл, в котором определен символ
	File string `json:"file" jsonschema:"File where the symbol is defined"`
	// Line - номер строки в файле
	Line int `json:"line" jsonschema:"Line number in the file"`
	// Exported - true, если символ экспортируется (начинается с заглавной буквы)
	Exported bool `json:"exported" jsonschema:"True if the symbol is exported (starts with capital letter)"`
}

// ListSymbolsOutput содержит результаты работы инструмента ListSymbols.
type ListSymbolsOutput struct {
	// Symbols - все найденные символы в указанном пакете
	Symbols []Symbol `json:"symbols" jsonschema:"All discovered symbols within the specified package"`
}

// ------------------ find references ------------------

// FindReferencesInput содержит входные данные для инструмента FindReferences.
type FindReferencesInput struct {
	// Dir - корневая директория Go-модуля
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Ident - имя символа для поиска ссылок
	Ident string `json:"ident" jsonschema:"Name of the symbol to find references for"`
	// File - опциональный относительный путь к файлу для ограничения поиска
	File string `json:"file,omitempty" jsonschema:"Optional relative file path to restrict the search"`
	// Kind - фильтр по типу символа (например, func, type, var, const)
	Kind string `json:"kind,omitempty" jsonschema:"Filter by symbol kind (e.g. func, type, var, const)"`
}

// Reference представляет ссылку на символ в Go-коде.
type Reference struct {
	// File - относительный путь к файлу, содержащему ссылку
	File string `json:"file" jsonschema:"Relative path to the file containing the reference"`
	// Line - номер строки ссылки
	Line int `json:"line" jsonschema:"Line number of the reference"`
	// Snippet - контекст кода, показывающий использование ссылки
	Snippet string `json:"snippet" jsonschema:"Code context showing the reference usage"`
}

// FindReferencesOutput содержит результаты работы инструмента FindReferences.
type FindReferencesOutput struct {
	// References - список всех найденных ссылок на данный идентификатор
	References []Reference `json:"references" jsonschema:"List of all found references to the given identifier"`
}

// ------------------ find definitions ------------------

// FindDefinitionsInput содержит входные данные для инструмента FindDefinitions.
type FindDefinitionsInput struct {
	// Dir - корневая директория Go-модуля
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Ident - имя символа для поиска определения
	Ident string `json:"ident" jsonschema:"Name of the symbol to locate its definition"`
	// File - опциональный относительный путь к файлу для ограничения поиска
	File string `json:"file,omitempty" jsonschema:"Optional relative file path to restrict the search"`
	// Kind - фильтр по типу символа (например, func, type, var, const)
	Kind string `json:"kind,omitempty" jsonschema:"Filter by symbol kind (e.g. func, type, var, const)"`
}

// Definition представляет определение символа в Go-коде.
type Definition struct {
	// File - относительный путь к файлу, где определен символ
	File string `json:"file" jsonschema:"Relative path to the file where the symbol is defined"`
	// Line - номер строки определения
	Line int `json:"line" jsonschema:"Line number of the definition"`
	// Snippet - сниппет кода, показывающий строку определения
	Snippet string `json:"snippet" jsonschema:"Code snippet showing the definition line"`
}

// FindDefinitionsOutput содержит результаты работы инструмента FindDefinitions.
type FindDefinitionsOutput struct {
	// Definitions - список найденных определений символа
	Definitions []Definition `json:"definitions" jsonschema:"List of found symbol definitions"`
}

// ------------------ list imports ------------------

// ListImportsInput содержит входные данные для инструмента ListImports.
type ListImportsInput struct {
	// Dir - корневая директория для сканирования Go-файлов
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go files"`
}

// Import представляет импорт пакета в Go-файле.
type Import struct {
	// Path - путь импортированного пакета
	Path string `json:"path" jsonschema:"Imported package path"`
	// File - файл, где объявлен импорт
	File string `json:"file" jsonschema:"File where the import is declared"`
	// Line - номер строки оператора импорта
	Line int `json:"line" jsonschema:"Line number of the import statement"`
}

// ListImportsOutput содержит результаты работы инструмента ListImports.
type ListImportsOutput struct {
	// Imports - все импорты, найденные в просканированных Go-файлах
	Imports []Import `json:"imports" jsonschema:"All imports found in scanned Go files"`
}

// ------------------ list interfaces ------------------

// ListInterfacesInput содержит входные данные для инструмента ListInterfaces.
type ListInterfacesInput struct {
	// Dir - корневая директория для сканирования Go-файлов
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go files"`
}

// InterfaceMethod представляет метод интерфейса.
type InterfaceMethod struct {
	// Name - имя метода
	Name string `json:"name" jsonschema:"Method name"`
	// Line - номер строки метода
	Line int `json:"line" jsonschema:"Line number of the method"`
}

// InterfaceInfo представляет информацию об интерфейсе.
type InterfaceInfo struct {
	// Name - имя интерфейса
	Name string `json:"name" jsonschema:"Interface name"`
	// File - файл, где определен интерфейс
	File string `json:"file" jsonschema:"File where the interface is defined"`
	// Line - номер строки объявления интерфейса
	Line int `json:"line" jsonschema:"Line number of the interface declaration"`
	// Methods - список методов, определенных в интерфейсе
	Methods []InterfaceMethod `json:"methods" jsonschema:"List of methods defined in the interface"`
}

// ListInterfacesOutput содержит результаты работы инструмента ListInterfaces.
type ListInterfacesOutput struct {
	// Interfaces - все интерфейсы, найденные в просканированных Go-файлах
	Interfaces []InterfaceInfo `json:"interfaces" jsonschema:"All interfaces found in scanned Go files"`
}

// ------------------ analyze complexity ------------------

// AnalyzeComplexityInput содержит входные данные для инструмента AnalyzeComplexity.
type AnalyzeComplexityInput struct {
	// Dir - корневая директория для сканирования Go-файлов
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go files"`
}

// FunctionComplexity представляет метрики сложности функции.
type FunctionComplexity struct {
	// Name - имя функции
	Name string `json:"name" jsonschema:"Function name"`
	// File - файл, где определена функция
	File string `json:"file" jsonschema:"File where the function is defined"`
	// Line - номер строки функции
	Line int `json:"line" jsonschema:"Line number of the function"`
	// Lines - общее количество строк в функции
	Lines int `json:"lines" jsonschema:"Total number of lines in the function"`
	// Nesting - максимальная глубина вложенности
	Nesting int `json:"nesting" jsonschema:"Maximum nesting depth"`
	// Cyclomatic - цикломатическая сложность
	Cyclomatic int `json:"cyclomatic" jsonschema:"Cyclomatic complexity value"`
}

// AnalyzeComplexityOutput содержит результаты работы инструмента AnalyzeComplexity.
type AnalyzeComplexityOutput struct {
	// Functions - рассчитанные метрики сложности для всех функций
	Functions []FunctionComplexity `json:"functions" jsonschema:"Calculated complexity metrics for all functions"`
}

// ------------------ dead code ------------------

// DeadCodeInput содержит входные данные для инструмента DeadCode.
type DeadCodeInput struct {
	// Dir - корневая директория для сканирования неиспользуемых символов
	Dir string `json:"dir" jsonschema:"Root directory to scan for unused symbols"`
	// IncludeExported - если true, включает экспортируемые символы, которые не используются
	IncludeExported bool `json:"includeExported,omitempty" jsonschema:"If true, include exported symbols that are unused"`
}

// DeadSymbol представляет неиспользуемый символ в Go-коде.
type DeadSymbol struct {
	// Name - имя символа
	Name string `json:"name" jsonschema:"Symbol name"`
	// Kind - тип символа (func, var, const, type)
	Kind string `json:"kind" jsonschema:"Symbol kind (func, var, const, type)"`
	// File - файл, где объявлен неиспользуемый символ
	File string `json:"file" jsonschema:"File where the unused symbol is declared"`
	// Line - номер строки символа
	Line int `json:"line" jsonschema:"Line number of the symbol"`
	// IsExported - true, если символ экспортируется (начинается с заглавной буквы)
	IsExported bool `json:"isExported" jsonschema:"True if the symbol is exported (starts with capital letter)"`
	// Package - пакет, где определен символ
	Package string `json:"package" jsonschema:"Package where the symbol is defined"`
}

// DeadCodeOutput содержит результаты работы инструмента DeadCode.
type DeadCodeOutput struct {
	// Unused - список неиспользуемых или мертвых символов кода
	Unused []DeadSymbol `json:"unused" jsonschema:"List of unused or dead code symbols"`
	// TotalCount - общее количество найденных неиспользуемых символов
	TotalCount int `json:"totalCount" jsonschema:"Total number of unused symbols found"`
	// ExportedCount - количество неиспользуемых экспортируемых символов
	ExportedCount int `json:"exportedCount" jsonschema:"Number of exported symbols that are unused"`
	// ByPackage - количество неиспользуемых символов, сгруппированное по пакетам
	ByPackage map[string]int `json:"byPackage" jsonschema:"Count of unused symbols grouped by package"`
}

// ------------------ rename symbol ------------------

// RenameSymbolInput содержит входные данные для инструмента RenameSymbol.
type RenameSymbolInput struct {
	// Dir - корневая директория Go-модуля
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// OldName - текущее имя символа для переименования
	OldName string `json:"oldName" jsonschema:"Current symbol name to rename"`
	// NewName - новое имя символа для применения
	NewName string `json:"newName" jsonschema:"New symbol name to apply"`
	// Kind - тип символа: func, var, const, type, package
	Kind string `json:"kind,omitempty" jsonschema:"Symbol kind: func, var, const, type, package"`
	// DryRun - если true, возвращает только предварительный просмотр изменений без записи файлов
	DryRun bool `json:"dryRun,omitempty" jsonschema:"If true, only return a diff preview without writing files"`
}

// FileDiff представляет дельту изменений в файле.
type FileDiff struct {
	// Path - путь к файлу, где произошли изменения
	Path string `json:"path" jsonschema:"File path where changes occurred"`
	// Diff - унифицированный дифф, показывающий изменения
	Diff string `json:"diff" jsonschema:"Unified diff showing changes"`
}

// RenameSymbolOutput содержит результаты работы инструмента RenameSymbol.
type RenameSymbolOutput struct {
	// ChangedFiles - список измененных файлов
	ChangedFiles []string `json:"changedFiles" jsonschema:"List of modified files"`
	// Diffs - результаты диффа, если использовался предварительный просмотр
	Diffs []FileDiff `json:"diffs,omitempty" jsonschema:"Diff results if dry run was used"`
	// Collisions - список конфликтов имен, препятствующих переименованию
	Collisions []string `json:"collisions,omitempty" jsonschema:"List of name conflicts preventing rename"`
}

// ------------------ analyze dependencies ------------------.

// AnalyzeDependenciesInput содержит входные данные для инструмента AnalyzeDependencies.
type AnalyzeDependenciesInput struct {
	// Dir - корневая директория для сканирования зависимостей пакетов
	Dir string `json:"dir" jsonschema:"Root directory to scan for package dependencies"`
}

// PackageDependency представляет информацию о зависимостях пакета.
type PackageDependency struct {
	// Package - путь к пакету
	Package string `json:"package" jsonschema:"Package path"`
	// Imports - список импортированных пакетов
	Imports []string `json:"imports" jsonschema:"List of imported packages"`
	// FanIn - количество других пакетов, которые импортируют этот пакет
	FanIn int `json:"fanIn" jsonschema:"Number of other packages that import this package"`
	// FanOut - количество пакетов, которые импортирует этот пакет
	FanOut int `json:"fanOut" jsonschema:"Number of packages this package imports"`
}

// AnalyzeDependenciesOutput содержит результаты работы инструмента AnalyzeDependencies.
type AnalyzeDependenciesOutput struct {
	// Dependencies - список пакетов и их зависимостей
	Dependencies []PackageDependency `json:"dependencies" jsonschema:"List of packages and their dependencies"`
	// Cycles - список циклов зависимостей, найденных в проекте
	Cycles [][]string `json:"cycles" jsonschema:"List of dependency cycles found in the project"`
}

// ------------------ find implementations ------------------.

// FindImplementationsInput содержит входные данные для инструмента FindImplementations.
type FindImplementationsInput struct {
	// Dir - корневая директория Go-модуля
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Name - имя интерфейса или типа для поиска реализаций
	Name string `json:"name" jsonschema:"Name of the interface or type to find implementations for"`
}

// Implementation представляет реализацию интерфейса.
type Implementation struct {
	// Type - имя реализующего типа
	Type string `json:"type" jsonschema:"Implementing type name"`
	// Interface - интерфейс, который реализуется
	Interface string `json:"interface" jsonschema:"Interface being implemented"`
	// File - файл, где определена реализация
	File string `json:"file" jsonschema:"File where the implementation is defined"`
	// Line - номер строки реализации
	Line int `json:"line" jsonschema:"Line number of the implementation"`
	// IsType - true, если это тип, реализующий интерфейс, false для встраивания интерфейса в интерфейс
	IsType bool `json:"isType" jsonschema:"True if this is a type implementing an interface, false for interface-to-interface embedding"`
}

// FindImplementationsOutput содержит результаты работы инструмента FindImplementations.
type FindImplementationsOutput struct {
	// Implementations - список найденных реализаций
	Implementations []Implementation `json:"implementations" jsonschema:"List of found implementations"`
}

// ------------------ metrics summary ------------------.

// MetricsSummaryInput содержит входные данные для инструмента MetricsSummary.
type MetricsSummaryInput struct {
	// Dir - корневая директория для сканирования метрик проекта
	Dir string `json:"dir" jsonschema:"Root directory to scan for project metrics"`
}

// MetricsSummaryOutput содержит результаты работы инструмента MetricsSummary.
type MetricsSummaryOutput struct {
	// PackageCount - общее количество пакетов
	PackageCount int `json:"packageCount" jsonschema:"Total number of packages"`
	// StructCount - общее количество структур
	StructCount int `json:"structCount" jsonschema:"Total number of structs"`
	// InterfaceCount - общее количество интерфейсов
	InterfaceCount int `json:"interfaceCount" jsonschema:"Total number of interfaces"`
	// FunctionCount - общее количество функций
	FunctionCount int `json:"functionCount" jsonschema:"Total number of functions"`
	// AverageCyclomatic - средняя цикломатическая сложность для всех функций
	AverageCyclomatic float64 `json:"averageCyclomatic" jsonschema:"Average cyclomatic complexity across all functions"`
	// DeadCodeCount - количество неиспользуемых символов
	DeadCodeCount int `json:"deadCodeCount" jsonschema:"Number of unused symbols"`
	// ExportedUnusedCount - количество неиспользуемых экспортируемых символов
	ExportedUnusedCount int `json:"exportedUnusedCount" jsonschema:"Number of exported symbols that are unused"`
	// LineCount - общее количество строк кода
	LineCount int `json:"lineCount" jsonschema:"Total lines of code"`
	// FileCount - общее количество Go-файлов
	FileCount int `json:"fileCount" jsonschema:"Total number of Go files"`
}

// ------------------ ast rewrite ------------------.

// ASTRewriteInput содержит входные данные для инструмента ASTRewrite.
type ASTRewriteInput struct {
	// Dir - корневая директория для выполнения перезаписи AST
	Dir string `json:"dir" jsonschema:"Root directory to perform AST rewriting"`
	// Find - паттерн для поиска (например, 'pkg.Func(x)')
	Find string `json:"find" jsonschema:"Pattern to find (e.g., 'pkg.Func(x)')"`
	// Replace - паттерн для замены (например, 'x.Method()')
	Replace string `json:"replace" jsonschema:"Pattern to replace with (e.g., 'x.Method()')"`
	// DryRun - если true, возвращает только предварительный просмотр изменений без записи файлов
	DryRun bool `json:"dryRun" jsonschema:"If true, only return a diff preview without writing files"`
}

// ASTRewriteOutput содержит результаты работы инструмента ASTRewrite.
type ASTRewriteOutput struct {
	// ChangedFiles - список файлов, которые были изменены
	ChangedFiles []string `json:"changedFiles" jsonschema:"List of files that were modified"`
	// Diffs - дифф изменений, если использовался предварительный просмотр
	Diffs []FileDiff `json:"diffs,omitempty" jsonschema:"Diff of changes if dry run was used"`
	// TotalChanges - общее количество внесенных изменений
	TotalChanges int `json:"totalChanges" jsonschema:"Total number of changes made"`
}

// ------------------ read func ------------------

// ReadFuncInput содержит входные данные для инструмента ReadFunc.
type ReadFuncInput struct {
	// Dir - корневая директория Go-модуля
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Name - имя функции или метода (например, 'List' или 'TaskService.List')
	Name string `json:"name" jsonschema:"Function or method name (e.g., 'List' or 'TaskService.List')"`
}

// FunctionSource представляет исходный код функции или метода в Go-коде.
type FunctionSource struct {
	// Name - имя функции
	Name string `json:"name" jsonschema:"Function name"`
	// Receiver - имя типа-получателя, если это метод (например, 'TaskService')
	Receiver string `json:"receiver,omitempty" jsonschema:"Receiver type name if this is a method (e.g., 'TaskService')"`
	// Package - путь к пакету, в котором определена функция
	Package string `json:"package" jsonschema:"Package path where the function is defined"`
	// File - относительный путь к файлу, где определена функция
	File string `json:"file" jsonschema:"Relative path to the file where the function is defined"`
	// StartLine - строка начала функции
	StartLine int `json:"startLine" jsonschema:"Starting line number of the function"`
	// EndLine - строка окончания функции
	EndLine int `json:"endLine" jsonschema:"Ending line number of the function"`
	// SourceCode - полный исходный код функции
	SourceCode string `json:"sourceCode" jsonschema:"Full source code of the function or method"`
}

// ReadFuncOutput содержит результаты работы инструмента ReadFunc.
type ReadFuncOutput struct {
	// Function - найденная функция с метаданными и исходным кодом
	Function FunctionSource `json:"function" jsonschema:"Extracted function with metadata and source code"`
}

// ------------------ read file ------------------

// ReadFileInput содержит входные данные для инструмента ReadFile.
type ReadFileInput struct {
	// Dir - корневая директория проекта (Go-модуль)
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// File - относительный путь к файлу, который нужно прочитать
	File string `json:"file" jsonschema:"Relative path to the Go source file to read"`
	// Mode - режим чтения: "raw" (только текст), "summary" (пакет, импорты, символы, строки), "ast" (полный AST-анализ)
	Mode string `json:"mode,omitempty" jsonschema:"Read mode: raw, summary, or ast"`
}

// ReadFileOutput содержит результаты работы инструмента ReadFile.
type ReadFileOutput struct {
	// File - путь к прочитанному файлу
	File string `json:"file" jsonschema:"File path that was read"`
	// Package - имя пакета, объявленного в файле
	Package string `json:"package,omitempty" jsonschema:"Declared Go package name"`
	// Imports - список импортированных пакетов
	Imports []Import `json:"imports,omitempty" jsonschema:"List of imported packages in the file"`
	// Symbols - функции, структуры, интерфейсы, константы и т.д.
	Symbols []Symbol `json:"symbols,omitempty" jsonschema:"List of declared symbols within the file"`
	// LineCount - общее количество строк в файле
	LineCount int `json:"lineCount" jsonschema:"Total number of lines in the file"`
	// Source - исходный код файла (если запрошен режим raw или ast)
	Source string `json:"source,omitempty" jsonschema:"Full source code of the file if requested"`
}

// ------------------ read struct ------------------

// ReadStructInput содержит входные данные для инструмента ReadStruct.
type ReadStructInput struct {
	// Dir - корневая директория Go-модуля
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Name - имя структуры (например, 'User' или 'models.User')
	Name string `json:"name" jsonschema:"Name of the struct to read (e.g., 'User' or 'models.User')"`
	// IncludeMethods - если true, возвращает также методы структуры
	IncludeMethods bool `json:"includeMethods,omitempty" jsonschema:"If true, also include methods of the struct"`
}

// StructField представляет отдельное поле структуры.
type StructField struct {
	// Name - имя поля
	Name string `json:"name" jsonschema:"Field name"`
	// Type - тип поля (например, string, int, time.Time)
	Type string `json:"type" jsonschema:"Field type"`
	// Tag - значение структурного тега (например, json:"id,omitempty")
	Tag string `json:"tag,omitempty" jsonschema:"Struct tag value"`
	// Doc - комментарий к полю, если есть
	Doc string `json:"doc,omitempty" jsonschema:"Field documentation comment"`
}

// StructInfo представляет объявление структуры.
type StructInfo struct {
	// Name - имя структуры
	Name string `json:"name" jsonschema:"Struct name"`
	// Package - имя пакета, в котором определена структура
	Package string `json:"package" jsonschema:"Package where the struct is defined"`
	// File - относительный путь к файлу, где определена структура
	File string `json:"file" jsonschema:"File where the struct is defined"`
	// Line - номер строки объявления структуры
	Line int `json:"line" jsonschema:"Line number where the struct is declared"`
	// Exported - true, если структура экспортируется
	Exported bool `json:"exported" jsonschema:"True if the struct is exported"`
	// Doc - документация над структурой (комментарий)
	Doc string `json:"doc,omitempty" jsonschema:"Struct documentation comment"`
	// Fields - список полей структуры
	Fields []StructField `json:"fields" jsonschema:"List of struct fields"`
	// Methods - список методов структуры, если IncludeMethods = true
	Methods []string `json:"methods,omitempty" jsonschema:"List of methods belonging to the struct"`
	// Source - исходный код объявления структуры
	Source string `json:"source" jsonschema:"Full struct source code"`
}

// ReadStructOutput содержит результаты работы инструмента ReadStruct.
type ReadStructOutput struct {
	// Struct - описание найденной структуры
	Struct StructInfo `json:"struct" jsonschema:"Description of the found struct"`
}
