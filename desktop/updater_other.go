//go:build !darwin

package main

import "fmt"

func prepareAndLaunchUpdate(_, _, _ string) error {
	return fmt.Errorf("当前系统暂不支持应用内更新")
}
