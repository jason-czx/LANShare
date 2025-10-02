package model

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/examples/lib/dev"
)

// Device 表示一个被扫描到的蓝牙设备
type Device struct {
	Addr string `json:"addr"`
	Name string `json:"name"`
	RSSI int    `json:"rssi"`
}

// BluetoothService 提供蓝牙扫描功能（平台实现放在 platform-specific 文件中）
type BluetoothService struct{}

func scanImpl(seconds int) ([]Device, error) {
	if seconds <= 0 {
		log.Printf("Scan called with non-positive seconds=%d, using default 5s", seconds)
		seconds = 5
	}

	log.Printf("creating BLE device (platform-specific)...")
	d, err := dev.NewDevice("default")
	if err != nil {
		log.Printf("failed to create device: %v", err)
		return nil, err
	}
	ble.SetDefaultDevice(d)

	log.Printf("starting scan for %d seconds", seconds)
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second))

	var devicesMu sync.Mutex
	devices := make(map[string]Device)

	// 发现回调
	advHandler := func(a ble.Advertisement) {
		addr := a.Addr().String()
		displayName := a.LocalName()
		if displayName == "" {
			return
		}
		// log each advertisement so we can see whether the callback fires
		log.Printf("adv received: addr=%s name=%q rssi=%d svcCount=%d", addr, a.LocalName(), a.RSSI(), len(a.Services()))
		devicesMu.Lock()
		defer devicesMu.Unlock()
		dvc := Device{Addr: addr, Name: displayName, RSSI: a.RSSI()}
		devices[addr] = dvc
	}

	// 开始扫描
	if err := ble.Scan(ctx, true, func(a ble.Advertisement) {
		advHandler(a)
	}, nil); err != nil {
		// Treat context deadline exceeded or cancellation as normal completion
		// so we can still collect and return any found devices. Only return
		// non-context errors as failures.
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			log.Printf("scan finished by timeout/cancel: %v", err)
		} else {
			log.Printf("scan finished with error: %v", err)
			return nil, err
		}
	}

	log.Printf("scan finished, collecting results...")

	// 从 map 收集结果
	devicesMu.Lock()
	defer devicesMu.Unlock()
	res := make([]Device, 0, len(devices))
	for _, v := range devices {
		res = append(res, v)
	}
	log.Printf("found %d unique devices", len(res))
	return res, nil
}

// Scan 在给定的秒数内扫描附近的蓝牙设备并返回设备列表。
// 如果平台不支持，会返回 error。
func (b *BluetoothService) Scan(seconds int) ([]Device, error) {
	return scanImpl(seconds)
}
