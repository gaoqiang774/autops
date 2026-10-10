package service

import (
	"net/url"
	"strings"

	"dodevops-api/api/cmdb/model"
)

type snmpVendorProfile struct {
	Name              string
	Module            string
	CPUMetricNames    []string
	MemoryMetricNames []string
	MemoryUsedMetrics []string
	MemoryFreeMetrics []string
	TempMetricNames   []string
}

func snmpResolveVendorProfile(device *model.CmdbSnmpDevice) snmpVendorProfile {
	if device == nil {
		return snmpResolveVendorProfileByText("", "")
	}
	return snmpResolveVendorProfileByText(device.DeviceVendor, device.DeviceModel)
}

func snmpResolveVendorProfileByText(vendor, deviceModel string) snmpVendorProfile {
	vendor = strings.ToLower(strings.TrimSpace(vendor))
	deviceModel = strings.ToLower(strings.TrimSpace(deviceModel))

	switch {
	case strings.Contains(vendor, "cisco") || strings.Contains(deviceModel, "cisco") || strings.Contains(deviceModel, "catalyst") || strings.Contains(deviceModel, "nexus"):
		return snmpVendorProfile{
			Name:   "cisco",
			Module: "cisco_switch",
			CPUMetricNames: []string{
				"cpmcputotal5minrev", "cpmcputotal5min",
				"cpmcputotal1minrev", "cpmcputotal1min",
				"cpmcputotal5secrev", "cpmcputotal5sec",
				"cpuusage", "systemcpuusage",
			},
			MemoryMetricNames: []string{
				"memoryusage", "systemmemusage", "systemmemoryusage",
			},
			MemoryUsedMetrics: []string{
				"ciscomemorypoolused", "cempmempoolused", "ciscoenhmemorypoolused",
			},
			MemoryFreeMetrics: []string{
				"ciscomemorypoolfree", "cempmempoolfree", "ciscoenhmemorypoolfree",
			},
			TempMetricNames: []string{
				"ciscoenvmontemperaturestatusvalue", "temperature", "temperaturecelsius",
			},
		}
	case strings.Contains(vendor, "huawei") || strings.Contains(deviceModel, "huawei") || strings.Contains(deviceModel, "s57") || strings.Contains(deviceModel, "s67"):
		return snmpVendorProfile{
			Name:   "huawei",
			Module: "huawei_switch",
			CPUMetricNames: []string{
				"systemcpuusage", "hwentitycpuusage", "cpuusage",
			},
			MemoryMetricNames: []string{
				"systemmemusage", "systemmemoryusage", "memoryusage", "hwentitymemusage",
			},
			TempMetricNames: []string{
				"temperaturereading", "temperature", "temperaturecelsius", "hwentitytemperature",
			},
		}
	case strings.Contains(vendor, "h3c") || strings.Contains(deviceModel, "h3c"):
		return snmpVendorProfile{
			Name:   "h3c",
			Module: "h3c_switch",
			CPUMetricNames: []string{
				"hh3centityextcpuusage", "hh3centitycpuusage", "cpuusage",
			},
			MemoryMetricNames: []string{
				"hh3centityextmemusage", "hh3centitymemusage", "memoryusage",
			},
			TempMetricNames: []string{
				"hh3centitytemperature", "hh3centityexttemperature", "temperature",
			},
		}
	default:
		return snmpVendorProfile{
			Name:   "generic",
			Module: "if_mib",
			CPUMetricNames: []string{
				"cpuusage", "systemcpuusage", "cpuload", "cpuutilization", "processorusage",
			},
			MemoryMetricNames: []string{
				"memoryusage", "systemmemusage", "systemmemoryusage", "memusage", "memoryutilization",
			},
			TempMetricNames: []string{
				"temperature", "temperaturereading", "temperatureprobereading", "temperaturecelsius",
			},
		}
	}
}

func snmpResolveRequestModule(device *model.CmdbSnmpDevice, currentModule string) string {
	profile := snmpResolveVendorProfile(device)
	currentModule = strings.TrimSpace(currentModule)
	if currentModule == "" || snmpIsGenericModule(currentModule) {
		return profile.Module
	}
	return currentModule
}

func snmpIsGenericModule(module string) bool {
	switch strings.ToLower(strings.TrimSpace(module)) {
	case "", "if_mib", "ifmib", "generic", "switch", "default":
		return true
	default:
		return false
	}
}

func snmpResolveRequestModules(device *model.CmdbSnmpDevice) []string {
	currentModule := snmpCurrentURLModule(device)
	modules := make([]string, 0, 3)
	seen := make(map[string]struct{})
	addModule := func(module string) {
		module = strings.TrimSpace(module)
		if module == "" {
			return
		}
		if _, exists := seen[module]; exists {
			return
		}
		seen[module] = struct{}{}
		modules = append(modules, module)
	}

	addModule(snmpResolveRequestModule(device, currentModule))
	addModule(currentModule)
	if len(modules) == 0 || !snmpIsGenericModule(modules[len(modules)-1]) {
		addModule("if_mib")
	}
	return modules
}

func snmpCurrentURLModule(device *model.CmdbSnmpDevice) string {
	if device == nil {
		return ""
	}
	normalized := ensureSNMPScheme(strings.TrimSpace(device.SnmpURL))
	if normalized == "" {
		return ""
	}
	parsedURL, err := url.Parse(normalized)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsedURL.Query().Get("module"))
}

func snmpMetricIn(name string, candidates []string) bool {
	for _, candidate := range candidates {
		if name == candidate {
			return true
		}
	}
	return false
}
