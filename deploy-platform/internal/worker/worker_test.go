package worker

import (
	"testing"

	"gantry/deploy-platform/internal/mq"
)

// TestValidMessage 验证完整消息通过且缺少 message_id 时被拒绝
func TestValidMessage(t *testing.T) {
	valid := mq.DeployMessage{MessageID: "m", DeploymentID: 1, AppID: 2, VersionID: 3}
	if !validMessage(valid) {
		t.Fatal("valid message rejected")
	}
	valid.MessageID = ""
	if validMessage(valid) {
		t.Fatal("message without message_id accepted")
	}
}
