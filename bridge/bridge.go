package bridge

import (
	"github.com/totalconnect/bridge/internal/rclone"
)

type Bridge = rclone.Bridge
type FileInfo = rclone.FileInfo
type Remote = rclone.Remote
type Progress = rclone.Progress

func NewBridge() *Bridge {
	return rclone.NewBridge()
}
