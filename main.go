package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"

	"flag"

	"github.com/afantree/astauto/logic"
	"golang.org/x/tools/go/ast/astutil"
)

var rootPath = flag.String("path", "./", "path to the directory or file to process")
var configPath = flag.String("conf", "./config.toml", "path to the config file")

// Usage is a replacement usage function for the flags package.
func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of astauto:\n")
	fmt.Fprintf(os.Stderr, "\tastauto -path directory\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = Usage
	flag.Parse()

	// 从TOML文件解析配置
	config, err := logic.ParseTOML(*configPath)
	if err != nil {
		log.Printf("从TOML解析失败，检查根目录下面的配置")
		os.Exit(1)
	}

	// 打印解析的配置
	printConfig(config)

	for _, rule := range config.Rules {
		// 处理Go文件修改
		if err := modifyGoFile(rule); err != nil {
			log.Fatalf("修改Go文件失败: %v", err)
		}
	}
}

// printConfig 打印配置信息
func printConfig(config *logic.Config) {
	fmt.Println("解析的配置:")
	for _, rule := range config.Rules {
		fmt.Printf("文件: %s\n", rule.File)
		fmt.Println("导入:")
		for _, imp := range rule.Imports {
			fmt.Printf("  - 路径: %s, 别名: %s\n", imp.Path, imp.Alias)
		}
		fmt.Println("结构体:")
		for _, st := range rule.Structs {
			fmt.Printf("  - 名称: %s\n", st.Name)
			fmt.Println("    字段:")
			for _, field := range st.Fields {
				fmt.Printf("      - 名称: %s, 类型: %s, 标签: %s\n", field.Name, field.Type, field.Tags)
			}
		}
	}
}

// modifyGoFile 根据配置修改Go文件
func modifyGoFile(rule *logic.Rule) error {
	var filename = filepath.Join(*rootPath, rule.File)
	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("文件 %s 不存在", filename)
		os.Exit(2)
	}

	fset := token.NewFileSet()
	// 解析Go源文件，保留注释
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("解析文件失败: %v", err)
	}

	// 添加导入
	for _, imp := range rule.Imports {
		path := imp.Path
		if imp.Alias != "" {
			// 使用别名导入
			if !astutil.AddNamedImport(fset, file, imp.Alias, path) {
				log.Printf("导入 %s 已经存在或不需要", path)
			}
			log.Printf("添加带别名的导入: %s as %s", path, imp.Alias)
		} else {
			// 普通导入
			if !astutil.AddImport(fset, file, path) {
				log.Printf("导入 %s 已经存在或不需要", path)
			}
			log.Printf("添加导入: %s", path)
		}
	}

	// 处理结构体
	astutil.Apply(file, nil, func(c *astutil.Cursor) bool {
		n := c.Node()

		// 检查节点是否为类型声明
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			// 查找匹配的结构体类型
			for _, st := range rule.Structs {
				if typeSpec.Name.Name == st.Name {
					// 确认该类型是一个结构体
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						for _, field := range st.Fields {
							// 检查字段是否已存在
							fieldExists := false
							for _, existingField := range structType.Fields.List {
								if len(existingField.Names) > 0 && existingField.Names[0].Name == field.Name {
									fieldExists = true
									log.Printf("字段 %s 已存在于结构体 %s 中，跳过添加\n", field.Name, st.Name)
									break
								}
							}

							// 如果字段不存在，则添加新字段
							if !fieldExists {
								// 创建新字段
								newField := &ast.Field{
									Names: []*ast.Ident{ast.NewIdent(field.Name)},
								}

								// 设置字段类型
								if field.Type[0] == '*' {
									// 指针类型
									parts := parseTypeParts(field.Type[1:])
									if len(parts) == 2 {
										newField.Type = &ast.StarExpr{
											X: &ast.SelectorExpr{
												X:   ast.NewIdent(parts[0]),
												Sel: ast.NewIdent(parts[1]),
											},
										}
									} else {
										newField.Type = &ast.StarExpr{
											X: ast.NewIdent(field.Type[1:]),
										}
									}
								} else {
									// 普通类型
									parts := parseTypeParts(field.Type)
									if len(parts) == 2 {
										newField.Type = &ast.SelectorExpr{
											X:   ast.NewIdent(parts[0]),
											Sel: ast.NewIdent(parts[1]),
										}
									} else {
										newField.Type = ast.NewIdent(field.Type)
									}
								}

								// 设置字段标签
								if field.Tags != "" {
									newField.Tag = &ast.BasicLit{
										Kind:  token.STRING,
										Value: "`" + field.Tags + "`",
									}
								}

								// 将新字段追加到结构体字段列表的末尾
								structType.Fields.List = append(structType.Fields.List, newField)
								log.Printf("成功添加字段 %s 到结构体 %s\n", field.Name, st.Name)
							}
						}
					}
				}
			}
		}
		return true
	})

	// 将修改后的 AST 写回文件
	outputFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %v", err)
	}
	defer outputFile.Close()

	// 使用 go/format 格式化输出，确保代码符合 gofmt 规范
	if err := format.Node(outputFile, fset, file); err != nil {
		return fmt.Errorf("格式化并写入文件失败: %v", err)
	}

	log.Printf("文件 %s 已成功修改并保存\n", rule.File)
	return nil
}

// parseTypeParts 解析类型字符串，返回包名和类型名（如果有）
func parseTypeParts(typeStr string) []string {
	for i, char := range typeStr {
		if char == '.' {
			return []string{typeStr[:i], typeStr[i+1:]}
		}
	}
	return []string{typeStr}
}
