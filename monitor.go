package libbox

import (
	"github.com/sagernet/sing/common/control"
	F "github.com/sagernet/sing/common/format"
	"github.com/v2fly/v2ray-core/v5/common/log"
)

var _ control.InterfaceFinder = (*networkManager)(nil)

type networkManager struct {
	control.DefaultInterfaceFinder
	interfaceName  string
	interfaceIndex int32
	iif            PlatformInterface
}

func newNetworkManager(iif PlatformInterface) *networkManager {
	return &networkManager{iif: iif}
}

func (m *networkManager) Start() error {
	return m.iif.StartDefaultInterfaceMonitor(m)
}

func (m *networkManager) Close() error {
	return m.iif.CloseDefaultInterfaceMonitor(m)
}

func (m *networkManager) UpdateDefaultInterface(interfaceName string, interfaceIndex int32, isExpensive bool, isConstrained bool) {
	_ = m.Update()
	if interfaceName != m.interfaceName || interfaceIndex != m.interfaceIndex {
		m.interfaceName = interfaceName
		m.interfaceIndex = interfaceIndex
		log.Record(&log.GeneralMessage{
			Severity: log.Severity_Info,
			Content:  F.ToString("updated default interface ", interfaceName, ", index ", interfaceIndex, ", expensive ", isExpensive, ", constrained ", isConstrained),
		})
	}
}
