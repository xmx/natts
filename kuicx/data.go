package kuicx

import "net"

type ClientInfo struct {
	ID   string           `json:"id"`
	Inet net.IP           `json:"inet"`
	MAC  net.HardwareAddr `json:"mac"` // 出口网卡 MAC 地址
	PID  int              `json:"pid"` // 进程 PID
}

func (ci ClientInfo) String() string {
	ip := ci.Inet.String()
	return ip + "(" + ci.ID + ")"
}

type HandshakeResult struct {
	Successful bool   `json:"successful"` // 是否握手成功
	Message    string `json:"message"`    // 握手失败的原因
}
