package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// LoadDataset 从 YAML 文件加载评测集。
func LoadDataset(path string) (*EvalDataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dataset file: %w", err)
	}

	var ds EvalDataset
	if err := yaml.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("parse dataset yaml: %w", err)
	}

	if err := validateDataset(&ds); err != nil {
		return nil, fmt.Errorf("validate dataset: %w", err)
	}

	return &ds, nil
}

// LoadDatasetVersion 加载指定版本的评测集（支持 latest symlink）。
func LoadDatasetVersion(basePath, version string) (*EvalDataset, error) {
	if version == "latest" {
		// 尝试解析 symlink
		link := filepath.Join(basePath, "latest")
		target, err := os.Readlink(link)
		if err == nil {
			version = filepath.Base(target)
		}
	}

	p := filepath.Join(basePath, version, "dataset.yaml")
	return LoadDataset(p)
}

// StratifiedSample 按分桶做分层抽样。
func StratifiedSample(items []EvalItem, maxItems int) []EvalItem {
	if maxItems <= 0 || len(items) <= maxItems {
		return items
	}

	// 按 bucket 分组
	buckets := make(map[string][]EvalItem)
	for _, item := range items {
		bucket := item.Bucket
		if bucket == "" {
			bucket = "default"
		}
		buckets[bucket] = append(buckets[bucket], item)
	}

	total := len(items)
	remaining := maxItems
	result := make([]EvalItem, 0, maxItems)

	// 按比例从每个 bucket 中抽样
	bucketNames := make([]string, 0, len(buckets))
	for name := range buckets {
		bucketNames = append(bucketNames, name)
	}
	sort.Strings(bucketNames)

	for i, name := range bucketNames {
		bucket := buckets[name]
		ratio := float64(len(bucket)) / float64(total)
		count := int(ratio * float64(maxItems))
		if i == len(bucketNames)-1 {
			count = remaining // 最后一个 bucket 拿剩余
		}
		if count > len(bucket) {
			count = len(bucket)
		}
		if count < 0 {
			count = 0
		}

		// deterministic: sort by item ID then take first count
		sort.Slice(bucket, func(a, b int) bool {
			return bucket[a].ID < bucket[b].ID
		})
		for j := 0; j < count; j++ {
			result = append(result, bucket[j])
		}
		remaining -= count
	}

	return result
}

// SaveBaseline 保存评分结果为基线。
func SaveBaseline(path string, report *EvalReport) error {
	data, err := yaml.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create baseline dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write baseline: %w", err)
	}
	return nil
}

// LoadBaseline 加载基线评分。
func LoadBaseline(path string) (*EvalReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}
	var report EvalReport
	if err := yaml.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	return &report, nil
}

// validateDataset 校验评测集合法性。
func validateDataset(ds *EvalDataset) error {
	if ds.ID == "" {
		return fmt.Errorf("dataset ID is required")
	}
	if ds.Version == "" {
		return fmt.Errorf("dataset version is required")
	}
	if len(ds.Items) == 0 {
		return fmt.Errorf("dataset must have at least one item")
	}
	for i, item := range ds.Items {
		if item.ID == "" {
			return fmt.Errorf("item[%d] ID is required", i)
		}
		if item.Domain == "" {
			return fmt.Errorf("item[%s] domain is required", item.ID)
		}
		if item.Dimension == "" {
			return fmt.Errorf("item[%s] dimension is required", item.ID)
		}
	}
	return nil
}
