package v1

import (
	"LANShare/model"
	"encoding/json"
	"fmt"
)

type BluetoothService struct{}

// BluetoothScanApi 扫描指定秒数并返回设备列表的 JSON 字符串。
func (g *BluetoothService) BluetoothScanApi(seconds int) (string, error) {
	svc := &model.BluetoothService{}
	devs, err := svc.Scan(seconds)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	b, err := json.Marshal(devs)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	fmt.Println(string(b))
	return string(b), nil
}
