package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	governanceConfig "github.com/Servora-Kit/servora/platform/config"

	kconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
)

// LoadBootstrap 加载服务启动配置，并返回可持续 watch 的 Kratos 配置实例。
// useEnvPrefix 为 true 时根据 serviceName 推导环境变量前缀（如 iam.service → IAM_），
// 仅读取带前缀的环境变量覆盖配置；为 false 时读取所有无前缀的环境变量。
func LoadBootstrap(configPath string, serviceName string, useEnvPrefix bool) (*conf.Bootstrap, kconfig.Config, error) {
	var envPrefix string
	if useEnvPrefix {
		envPrefix = strings.ToUpper(strings.TrimSuffix(serviceName, ".service")) + "_"
	}
	initialSources, err := buildFileSources(configPath)
	if err != nil {
		return nil, nil, err
	}

	tempConfig := kconfig.New(
		kconfig.WithSource(initialSources...),
		kconfig.WithResolveActualTypes(true),
	)
	if err := tempConfig.Load(); err != nil {
		return nil, nil, err
	}

	var bc conf.Bootstrap
	if err := tempConfig.Scan(&bc); err != nil {
		_ = tempConfig.Close()
		return nil, nil, err
	}

	var configCenterSource kconfig.Source
	if cfg := bc.Config; cfg != nil {
		switch v := cfg.Config.(type) {
		case *conf.Config_Nacos:
			configCenterSource = governanceConfig.NewNacosConfigSource(v.Nacos)
		case *conf.Config_Consul:
			configCenterSource = governanceConfig.NewConsulConfigSource(v.Consul)
		case *conf.Config_Etcd:
			configCenterSource = governanceConfig.NewEtcdConfigSource(v.Etcd)
		}
	}
	_ = tempConfig.Close()

	finalSources, err := buildFileSources(configPath)
	if err != nil {
		return nil, nil, err
	}
	if configCenterSource != nil {
		finalSources = append(finalSources, configCenterSource)
	}
	finalSources = append(finalSources, env.NewSource(envPrefix))

	c := kconfig.New(
		kconfig.WithSource(finalSources...),
		kconfig.WithResolveActualTypes(true),
	)
	if err := c.Load(); err != nil {
		return nil, nil, err
	}
	if err := c.Scan(&bc); err != nil {
		_ = c.Close()
		return nil, nil, err
	}

	return &bc, c, nil
}

// buildFileSources 将单文件或目录路径转换为 Kratos 可加载的文件配置源列表。
//
// 之所以在这里显式展开目录，而不是直接把目录交给 file.NewSource，
// 是因为运行期 watch 目录源时会触发 "read <dir>: is a directory" 类错误。
// 预先展开为多个文件源后，既能保留 Kratos 的多 Source 合并语义，
// 又能让 watch 针对具体文件工作。
func buildFileSources(configPath string) ([]kconfig.Source, error) {
	info, err := os.Stat(configPath)
	if err != nil {
		return nil, fmt.Errorf("stat config path: %w", err)
	}

	if !info.IsDir() {
		return []kconfig.Source{file.NewSource(configPath)}, nil
	}

	entries, err := os.ReadDir(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config dir: %w", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(configPath, entry.Name()))
	}

	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("config dir %q contains no files", configPath)
	}

	sources := make([]kconfig.Source, 0, len(paths))
	for _, path := range paths {
		sources = append(sources, file.NewSource(path))
	}

	return sources, nil
}
