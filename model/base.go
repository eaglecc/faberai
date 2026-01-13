package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CreatedAt time.Time      `json:"createdAt" gorm:"column:created_at;not null"`
	UpdatedAt time.Time      `json:"updatedAt" gorm:"column:updated_at;not null"`
	DeletedAt gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"column:deleted_at;index"`
}

// JSON type for PostgreSQL jsonb fields
type JSON map[string]interface{}

// NewJSON 将任意类型转换为JSON类型
func NewJSON(v interface{}) (JSON, error) {
	if v == nil {
		return make(JSON), nil
	}

	// 先将对象转换为JSON字节
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	// 再将JSON字节转换为map[string]interface{}
	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	return JSON(result), nil
}

// Scan 实现 sql.Scanner 接口，用于从数据库读取 JSON 数据
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}

	// 将数据库中的值转换为字节切片
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into JSON", value)
	}

	// 解析 JSON 数据
	result := make(map[string]interface{})
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*j = JSON(result)
	return nil
}

// Value 实现 driver.Valuer 接口，用于将 JSON 数据写入数据库
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return make(JSON), nil
	}

	// 将 JSON 转换为字节切片
	return json.Marshal(j)
}

// ToModelParams 将JSON类型的ModelParameters转换为ModelParams结构体
func (j JSON) ToModelParams() ModelsParams {
	params := ModelsParams{}

	if maxTokens, ok := j["maxTokens"].(float64); ok {
		params.MaxTokens = int(maxTokens)
	}

	if temperature, ok := j["temperature"].(float64); ok {
		params.Temperature = temperature
	}

	if topP, ok := j["topP"].(float64); ok {
		params.TopP = topP
	}

	if n, ok := j["n"].(float64); ok {
		params.N = int(n)
	}

	if stop, ok := j["stop"].([]any); ok {
		params.Stop = stop
	}

	if presencePenalty, ok := j["presencePenalty"].(float64); ok {
		params.PresencePenalty = presencePenalty
	}

	if frequencyPenalty, ok := j["frequencyPenalty"].(float64); ok {
		params.FrequencyPenalty = frequencyPenalty
	}

	return params
}
