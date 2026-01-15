package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestWeatherTool(t *testing.T) {
	// 创建一个天气查询工具实例
	weatherTool := NewWeatherTool(&WeatherConfig{
		ApiKey: ApiKey,
	})

	// 创建一个工具调用参数
	params := map[string]string{
		"city":       "北京",
		"extensions": "all",
	}
	marshal, _ := json.Marshal(params)
	invokableRun, err := weatherTool.InvokableRun(context.Background(), string(marshal))
	if err != nil {
		t.Errorf("InvokableRun() error = %v", err)
	}
	t.Logf("InvokableRun() = %v", invokableRun)

}
