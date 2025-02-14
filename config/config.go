// Package config 实现了 Trojan-Go 的配置管理功能
// 支持 JSON 和 YAML 格式的配置文件解析
// 使用 context 来存储和传递配置信息
package config

import (
	"context"
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// creators 存储了所有已注册的配置创建器
// 键是配置模块的名称，值是对应的创建器函数
var creators = make(map[string]Creator)

// Creator 是创建模块默认配置结构的函数类型
// 每个模块都需要提供一个创建器来生成自己的配置结构
type Creator func() interface{}

// RegisterConfigCreator 注册一个配置结构用于解析
// name: 模块名称
// creator: 创建配置结构的函数
func RegisterConfigCreator(name string, creator Creator) {
	name += "_CONFIG"
	creators[name] = creator
}

// parseJSON 解析 JSON 格式的配置数据
// 会为每个已注册的模块解析配置
func parseJSON(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for name, creator := range creators {
		config := creator()
		if err := json.Unmarshal(data, config); err != nil {
			return nil, err
		}
		result[name] = config
	}
	return result, nil
}

// parseYAML 解析 YAML 格式的配置数据
// 会为每个已注册的模块解析配置
func parseYAML(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for name, creator := range creators {
		config := creator()
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, err
		}
		result[name] = config
	}
	return result, nil
}

// WithJSONConfig 使用 JSON 配置数据创建新的上下文
// 解析配置数据并将结果存储在上下文中
func WithJSONConfig(ctx context.Context, data []byte) (context.Context, error) {
	var configs map[string]interface{}
	var err error
	configs, err = parseJSON(data)
	if err != nil {
		return ctx, err
	}
	for name, config := range configs {
		ctx = context.WithValue(ctx, name, config)
	}
	return ctx, nil
}

// WithYAMLConfig 使用 YAML 配置数据创建新的上下文
// 解析配置数据并将结果存储在上下文中
func WithYAMLConfig(ctx context.Context, data []byte) (context.Context, error) {
	var configs map[string]interface{}
	var err error
	configs, err = parseYAML(data)
	if err != nil {
		return ctx, err
	}
	for name, config := range configs {
		ctx = context.WithValue(ctx, name, config)
	}
	return ctx, nil
}

// WithConfig 将单个配置对象添加到上下文中
// name: 模块名称
// cfg: 配置对象
func WithConfig(ctx context.Context, name string, cfg interface{}) context.Context {
	name += "_CONFIG"
	return context.WithValue(ctx, name, cfg)
}

// FromContext 从上下文中提取配置
// 返回指定模块名称对应的配置对象
func FromContext(ctx context.Context, name string) interface{} {
	return ctx.Value(name + "_CONFIG")
}
