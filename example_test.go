package zaphelper

import (
	"time"
)

func Example() {
	InitLogger("/tmp", false, time.Local)
	logger := GetLogger("helloworld")
	logger.Infow("hello log", "key", "value")
	RotateLog()
}
