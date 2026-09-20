"use client";
import React, { useState, useMemo } from "react";
import {
  IdcRoom,
  IdcCabinet,
  PhysicalHost,
  SwitchDevice,
  BusinessModel
} from "../cmdbData";

interface IdcManagementProps {
  rooms: IdcRoom[];
  cabinets: IdcCabinet[];
  hosts: PhysicalHost[];
  switches: SwitchDevice[];
  businesses: BusinessModel[];
  onAddRoom: (room: IdcRoom) => void;
  onDeleteRoom: (roomId: string) => void;
  onAddCabinet: (cab: IdcCabinet) => void;
  onDeleteCabinet: (cabId: string) => void;
  onAddHost: (host: PhysicalHost) => void;
  onDeleteHost: (hostId: string) => void;
  onNavigateToBusiness?: (bizId: string) => void;
}

export default function IdcManagement({
  rooms,
  cabinets,
  hosts,
  switches,
  businesses,
  onAddRoom,
  onDeleteRoom,
  onAddCabinet,
  onDeleteCabinet,
  onAddHost,
  onDeleteHost,
  onNavigateToBusiness
}: IdcManagementProps) {
  // Navigation & Selection State
  const [selectedRoomId, setSelectedRoomId] = useState<string | null>(rooms[0]?.id || "room-1");
  const [selectedCabinetId, setSelectedCabinetId] = useState<string | null>(cabinets[0]?.id || "cab-llq-01");
  const [activeTab, setActiveTab] = useState<"rack" | "floor" | "table" | "hardware">("rack");
  const [selectedHostId, setSelectedHostId] = useState<string>(hosts[0]?.id || "phy-1");
  const [treeSearch, setTreeSearch] = useState("");
  const [tableSearch, setTableSearch] = useState("");
  const [brandFilter, setBrandFilter] = useState("全部");

  // Modals state
  const [showRoomModal, setShowRoomModal] = useState(false);
  const [showCabinetModal, setShowCabinetModal] = useState(false);
  const [showMountModal, setShowMountModal] = useState(false);
  const [mountSlotPrefill, setMountSlotPrefill] = useState<number>(1);
  const [slotConflictMsg, setSlotConflictMsg] = useState("");

  // Room Form State
  const [roomForm, setRoomForm] = useState({
    name: "",
    code: "",
    operator: "BGP多线" as IdcRoom["operator"],
    city: "北京",
    address: "",
    level: "T3+" as IdcRoom["level"],
    contact: "",
    phone: "",
    remark: ""
  });

  // Cabinet Form State
  const [cabinetForm, setCabinetForm] = useState({
    roomId: "room-1",
    name: "",
    code: "",
    maxPower: 5.0,
    manager: "",
    row: "Row A",
    remark: ""
  });

  // Host Mount Form State
  const [mountForm, setMountForm] = useState({
    assetNo: "",
    hostname: "",
    roomId: "room-1",
    cabinetId: "cab-101",
    startU: 1,
    uHeight: 2,
    ip: "",
    bmcIp: "",
    brand: "Dell" as PhysicalHost["brand"],
    model: "PowerEdge R740",
    os: "CentOS 7.9 (Linux 3.10)",
    cpu: "2× Intel Xeon Silver 4210R (20C/40T)",
    memory: "128 GB DDR4",
    disk: "2× 960GB SSD + 4× 4TB SATA",
    role: "应用生产服务器",
    businessId: "biz-1"
  });

  // Filtered Cabinets for current selection
  const currentRoom = useMemo(() => rooms.find(r => r.id === selectedRoomId), [rooms, selectedRoomId]);
  const currentCabinet = useMemo(() => cabinets.find(c => c.id === selectedCabinetId), [cabinets, selectedCabinetId]);

  // All devices in the current selected cabinet (both hosts and switches)
  const cabinetHosts = useMemo(() => {
    return hosts.filter(h => h.cabinetId === selectedCabinetId);
  }, [hosts, selectedCabinetId]);

  const cabinetSwitches = useMemo(() => {
    return switches.filter(s => s.cabinetId === selectedCabinetId);
  }, [switches, selectedCabinetId]);

  // Selected server for Hardware Monitor
  const selectedHost = useMemo(() => {
    return hosts.find(h => h.id === selectedHostId) || hosts[0] || null;
  }, [hosts, selectedHostId]);

  // Overall statistics
  const totalRooms = rooms.length;
  const totalCabinets = cabinets.length;
  const totalHosts = hosts.length;
  const totalSwitches = switches.length;
  const totalAssets = totalHosts + totalSwitches;
  const avgUUsage = Math.round(cabinets.reduce((acc, c) => acc + (c.usedU / c.totalU) * 100, 0) / (cabinets.length || 1));
  const totalPower = (cabinets.reduce((acc, c) => acc + c.currentPower, 0)).toFixed(1);

  // Tree filter
  const filteredRooms = useMemo(() => {
    if (!treeSearch.trim()) return rooms;
    const q = treeSearch.toLowerCase();
    return rooms.filter(r => 
      r.name.toLowerCase().includes(q) || 
      r.code.toLowerCase().includes(q) ||
      cabinets.some(c => c.roomId === r.id && c.name.toLowerCase().includes(q))
    );
  }, [rooms, cabinets, treeSearch]);

  // Table filter
  const filteredTableHosts = useMemo(() => {
    return hosts.filter(h => {
      const matchRoom = !selectedRoomId || h.roomId === selectedRoomId;
      const matchCab = !selectedCabinetId || h.cabinetId === selectedCabinetId;
      const matchBrand = brandFilter === "全部" || h.brand === brandFilter;
      const matchText = !tableSearch.trim() || 
        h.hostname.toLowerCase().includes(tableSearch.toLowerCase()) || 
        h.ip.includes(tableSearch) || 
        h.assetNo.toLowerCase().includes(tableSearch.toLowerCase());
      return matchRoom && matchCab && matchBrand && matchText;
    });
  }, [hosts, selectedRoomId, selectedCabinetId, brandFilter, tableSearch]);

  // U-Slot collision check
  function checkSlotCollision(cabinetId: string, startU: number, height: number): string | null {
    const endU = startU + height - 1;
    if (startU < 1 || endU > 42) {
      return `U位超出范围 (1U - 42U)，当前选择为 ${startU}U - ${endU}U`;
    }
    // Check hosts in cabinet
    for (const h of hosts.filter(x => x.cabinetId === cabinetId)) {
      const hEnd = (h.startU ?? 0) + (h.uHeight ?? 1) - 1;
      if (!(endU < (h.startU ?? 999) || startU > hEnd)) {
        return `U位 ${startU}U - ${endU}U 与在架服务器 [${h.hostname}] (${h.startU ?? '?'}U-${hEnd}U) 冲突！`;
      }
    }
    // Check switches in cabinet
    for (const s of switches.filter(x => x.cabinetId === cabinetId)) {
      const sEnd = (s.startU ?? 0) + (s.uHeight ?? 1) - 1;
      if (!(endU < (s.startU ?? 999) || startU > sEnd)) {
        return `U位 ${startU}U - ${endU}U 与交换机 [${s.name}] (${s.startU ?? '?'}U-${sEnd}U) 冲突！`;
      }
    }
    return null;
  }

  // Pre-open Mount modal for a specific empty U slot
  function handleEmptySlotClick(slotNum: number) {
    if (!selectedCabinetId) return;
    setMountSlotPrefill(slotNum);
    setMountForm(prev => ({
      ...prev,
      roomId: selectedRoomId || "room-1",
      cabinetId: selectedCabinetId,
      startU: slotNum,
      uHeight: 2,
      assetNo: `SRV-AUTO-${Math.floor(100 + Math.random() * 900)}`,
      hostname: `prod-node-${slotNum}u`,
      ip: `10.100.10.${slotNum + 30}`,
      bmcIp: `192.168.100.${slotNum + 30}`
    }));
    setSlotConflictMsg("");
    setShowMountModal(true);
  }

  // Submit new Host Mount
  function handleMountSubmit(e: React.FormEvent) {
    e.preventDefault();
    const conflict = checkSlotCollision(mountForm.cabinetId, mountForm.startU, mountForm.uHeight);
    if (conflict) {
      setSlotConflictMsg(conflict);
      return;
    }

    const newHost: PhysicalHost = {
      id: `host-${Date.now()}`,
      assetNo: mountForm.assetNo || `SRV-${Date.now().toString().slice(-4)}`,
      hostname: mountForm.hostname || "new-srv-node",
      name: mountForm.hostname || `new-srv-node-${Date.now().toString().slice(-4)}`,
      roomId: mountForm.roomId,
      cabinetId: mountForm.cabinetId,
      startU: Number(mountForm.startU),
      uHeight: Number(mountForm.uHeight),
      ip: mountForm.ip || "10.100.10.50",
      bmcIp: mountForm.bmcIp || "192.168.100.50",
      brand: mountForm.brand,
      model: mountForm.model,
      os: mountForm.os,
      cpu: mountForm.cpu,
      memory: mountForm.memory,
      disk: mountForm.disk,
      role: mountForm.role,
      businessId: mountForm.businessId,
      status: "running",
      powerWatts: 350,
      cpuTemp: 42,
      boardTemp: 33,
      inletTemp: 22,
      fanRpm: "4300 RPM (正常)",
      psu1Status: "正常 (185W)",
      psu2Status: "正常 (175W)",
      raidStatus: "RAID1 正常",
      agentOnline: true,
      updated: new Date().toLocaleString("zh-CN", { hour12: false }).slice(0, 16)
    };

    onAddHost(newHost);
    setShowMountModal(false);
  }

  // Submit new Room
  function handleRoomSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!roomForm.name.trim()) return;
    const newRoom: IdcRoom = {
      id: `room-${Date.now()}`,
      name: roomForm.name,
      code: roomForm.code || `ROOM-${Date.now().toString().slice(-3)}`,
      operator: roomForm.operator,
      city: roomForm.city,
      address: roomForm.address || "自建数据中心",
      level: roomForm.level,
      cabinetCount: 12,
      contact: roomForm.contact || "运维部",
      phone: roomForm.phone || "010-88889999",
      status: "正常运行",
      remark: roomForm.remark || "北控伟仕标准自建机房"
    };
    onAddRoom(newRoom);
    setSelectedRoomId(newRoom.id);
    setShowRoomModal(false);
  }

  // Submit new Cabinet
  function handleCabinetSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!cabinetForm.name.trim()) return;
    const newCabinet: IdcCabinet = {
      id: `cab-${Date.now()}`,
      roomId: cabinetForm.roomId,
      name: cabinetForm.name,
      code: cabinetForm.code || `CAB-${Date.now().toString().slice(-4)}`,
      totalU: 42,
      usedU: 0,
      maxPower: cabinetForm.maxPower || 5.0,
      currentPower: 0.0,
      temperature: 21.5,
      status: "空闲",
      manager: cabinetForm.manager || "王工",
      row: cabinetForm.row || "Row A",
      remark: cabinetForm.remark || "标准 42U 拓展机柜"
    };
    onAddCabinet(newCabinet);
    setSelectedCabinetId(newCabinet.id);
    setShowCabinetModal(false);
  }

  // Delete safety checks
  function handleDeleteRoomSafe(roomId: string, name: string) {
    const hasCabs = cabinets.some(c => c.roomId === roomId);
    if (hasCabs) {
      alert(`无法删除机房【${name}】：机房内尚有正在运行的机柜或在架资产，请先清空或迁移机柜！`);
      return;
    }
    if (confirm(`确定要彻底删除机房【${name}】吗？`)) {
      onDeleteRoom(roomId);
      if (selectedRoomId === roomId) {
        setSelectedRoomId(rooms[0]?.id || null);
      }
    }
  }

  function handleDeleteCabinetSafe(cabId: string, name: string) {
    const hasAssets = hosts.some(h => h.cabinetId === cabId) || switches.some(s => s.cabinetId === cabId);
    if (hasAssets) {
      alert(`无法删除机柜【${name}】：机柜内尚有在架服务器或网络设备，请先将设备下架！`);
      return;
    }
    if (confirm(`确定要彻底删除机柜【${name}】吗？`)) {
      onDeleteCabinet(cabId);
      if (selectedCabinetId === cabId) {
        setSelectedCabinetId(cabinets[0]?.id || null);
      }
    }
  }

  return (
    <div className="cmdb-container">
      {/* Top Level Metric Strip */}
      <section className="cmdb-kpi-strip">
        <div className="kpi-card">
          <div className="kpi-icon blue">▦</div>
          <div className="kpi-body">
            <span className="kpi-title">机房总数</span>
            <span className="kpi-value">{totalRooms} <small>个数据中心</small></span>
          </div>
        </div>
        <div className="kpi-card">
          <div className="kpi-icon green">▥</div>
          <div className="kpi-body">
            <span className="kpi-title">在管机柜</span>
            <span className="kpi-value">{totalCabinets} <small>个标准机柜</small></span>
          </div>
        </div>
        <div className="kpi-card">
          <div className="kpi-icon purple">▣</div>
          <div className="kpi-body">
            <span className="kpi-title">在架物理资产</span>
            <span className="kpi-value">{totalAssets} <small>台设备</small></span>
          </div>
        </div>
        <div className="kpi-card">
          <div className="kpi-icon gold">◉</div>
          <div className="kpi-body">
            <span className="kpi-title">平均U位利用率</span>
            <span className="kpi-value">{avgUUsage}% <small>空间配比</small></span>
          </div>
        </div>
        <div className="kpi-card">
          <div className="kpi-icon cyan">⚡</div>
          <div className="kpi-body">
            <span className="kpi-title">当前总负荷功率</span>
            <span className="kpi-value">{totalPower} <small>kW 实时功耗</small></span>
          </div>
        </div>
      </section>

      {/* Main Dual-Column Panel */}
      <div className="cmdb-main-layout">
        {/* Left Column: IDC Tree Hierarchy */}
        <aside className="idc-tree-panel">
          <header className="idc-tree-header">
            <div className="idc-tree-title">
              <h3><span>☷</span> 机房空间树</h3>
              <span>{rooms.length} 机房 · {cabinets.length} 机柜</span>
            </div>
            <div className="tree-actions-row">
              <button className="tree-btn" title="新建机房" onClick={() => setShowRoomModal(true)}>
                + 机房
              </button>
              <button className="tree-btn" title="新建机柜" onClick={() => setShowCabinetModal(true)}>
                + 机柜
              </button>
            </div>
          </header>

          <div className="idc-tree-search">
            <input 
              placeholder="搜索机房 / 机柜编号..." 
              value={treeSearch}
              onChange={e => setTreeSearch(e.target.value)}
            />
          </div>

          <div className="idc-tree-scroll">
            <div 
              className={`tree-all-node ${selectedRoomId === null ? "active" : ""}`}
              onClick={() => { setSelectedRoomId(null); setSelectedCabinetId(null); }}
            >
              <span>🌐 全部机房与资产</span>
              <span className="room-badge">{totalAssets} 台</span>
            </div>

            {filteredRooms.map(room => {
              const roomCabs = cabinets.filter(c => c.roomId === room.id);
              const isRoomActive = selectedRoomId === room.id;

              return (
                <div key={room.id} className="tree-room-node">
                  <div 
                    className={`room-node-bar ${isRoomActive ? "selected-room" : ""}`}
                    onClick={() => {
                      setSelectedRoomId(room.id);
                      if (roomCabs.length > 0 && (!selectedCabinetId || !roomCabs.some(c => c.id === selectedCabinetId))) {
                        setSelectedCabinetId(roomCabs[0].id);
                      }
                    }}
                  >
                    <div className="room-node-left">
                      <span>🏢</span>
                      <span>{room.name}</span>
                    </div>
                    <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                      <span className="room-badge">{roomCabs.length} 柜</span>
                      <button 
                        style={{ background: "none", border: 0, color: "#94a3b8", cursor: "pointer", fontSize: 12 }}
                        title="删除机房"
                        onClick={(e) => { e.stopPropagation(); handleDeleteRoomSafe(room.id, room.name); }}
                      >
                        ×
                      </button>
                    </div>
                  </div>

                  {isRoomActive && (
                    <div className="cabinet-sublist">
                      {roomCabs.map(cab => {
                        const isCabActive = selectedCabinetId === cab.id;
                        const cabDeviceCount = hosts.filter(h => h.cabinetId === cab.id).length + switches.filter(s => s.cabinetId === cab.id).length;

                        return (
                          <div 
                            key={cab.id} 
                            className={`cabinet-node-item ${isCabActive ? "active" : ""}`}
                            onClick={() => setSelectedCabinetId(cab.id)}
                          >
                            <span>▥ {cab.name}</span>
                            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                              <span className="u-tag">{cab.usedU}/42U ({cabDeviceCount}台)</span>
                              <button 
                                style={{ background: "none", border: 0, color: "#cbd5e1", cursor: "pointer", fontSize: 11 }}
                                title="删除机柜"
                                onClick={(e) => { e.stopPropagation(); handleDeleteCabinetSafe(cab.id, cab.name); }}
                              >
                                ×
                              </button>
                            </div>
                          </div>
                        );
                      })}
                      {roomCabs.length === 0 && (
                        <div style={{ fontSize: 11, color: "#94a3b8", padding: "4px 8px" }}>
                          暂无机柜，点击右上角 "+ 机柜"
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </aside>

        {/* Right Column: Interactive Multi-View Area */}
        <main className="idc-content-panel">
          {/* Navigation Tab Bar */}
          <nav className="idc-nav-tabs">
            <div className="tabs-group">
              <button 
                className={`idc-tab-btn ${activeTab === "rack" ? "active" : ""}`}
                onClick={() => setActiveTab("rack")}
              >
                <span>▥</span> 42U 机柜立面图
              </button>
              <button 
                className={`idc-tab-btn ${activeTab === "floor" ? "active" : ""}`}
                onClick={() => setActiveTab("floor")}
              >
                <span>▣</span> 机房机柜平面分布
              </button>
              <button 
                className={`idc-tab-btn ${activeTab === "table" ? "active" : ""}`}
                onClick={() => setActiveTab("table")}
              >
                <span>▤</span> 物理资产清单
              </button>
              <button 
                className={`idc-tab-btn ${activeTab === "hardware" ? "active" : ""}`}
                onClick={() => setActiveTab("hardware")}
              >
                <span>⚡</span> 硬件带外与传感器
              </button>
            </div>

            <div className="tab-actions">
              <button 
                className="btn-primary"
                onClick={() => {
                  setMountSlotPrefill(1);
                  setMountForm(prev => ({
                    ...prev,
                    roomId: selectedRoomId || "room-1",
                    cabinetId: selectedCabinetId || "cab-101",
                    startU: 1
                  }));
                  setSlotConflictMsg("");
                  setShowMountModal(true);
                }}
              >
                <span>＋</span> 新增设备上架
              </button>
            </div>
          </nav>

          {/* Viewport content */}
          <div className="idc-view-viewport">
            {/* VIEW 1: 42U RACK ELEVATION */}
            {activeTab === "rack" && (
              <div className="rack-elevation-container">
                {/* 42U Metal Rack Framework */}
                <div className="rack-frame">
                  <div className="rack-top-banner">
                    <div>
                      <b>{currentCabinet ? currentCabinet.name : "未选择机柜"}</b>
                      <span style={{ marginLeft: 8 }}>({currentRoom ? currentRoom.name : "全部机房"})</span>
                    </div>
                    <div>
                      <span>42U 标准冷热通道封闭式机架</span>
                    </div>
                  </div>

                  <div className="rack-slots-column">
                    {/* Render 42 slots from 42 down to 1 */}
                    {Array.from({ length: 42 }, (_, i) => 42 - i).map(slotNum => {
                      // Check if a host starts or occupies this slot
                      const mountedHost = cabinetHosts.find(h => slotNum >= (h.startU ?? 0) && slotNum < (h.startU ?? 0) + (h.uHeight ?? 1));
                      const mountedSwitch = cabinetSwitches.find(s => slotNum >= (s.startU ?? 0) && slotNum < (s.startU ?? 0) + (s.uHeight ?? 1));

                      // If a multi-U device starts at a different slot and covers this slot, only render on the top-most slot or render spanning
                      if (mountedHost) {
                        // Top slot of this device
                        const isDeviceTop = slotNum === ((mountedHost.startU ?? 0) + (mountedHost.uHeight ?? 1) - 1);
                        if (!isDeviceTop) return null; // skipped because parent block spans

                        const heightPx = (mountedHost.uHeight ?? 1) * 30 - 2;
                        const isSelected = selectedHostId === mountedHost.id;

                        let brandClass = "device-dell";
                        if (mountedHost.brand === "华为") brandClass = "device-huawei";
                        if (mountedHost.brand === "浪潮") brandClass = "device-inspur";

                        return (
                          <div key={slotNum} className="rack-u-row" style={{ height: heightPx }}>
                            <div className="u-num-tag">{(mountedHost.startU ?? 0) + (mountedHost.uHeight ?? 1) - 1}U</div>
                            <div 
                              className={`slot-occupied ${brandClass} ${isSelected ? "selected" : ""}`}
                              style={{ height: "100%" }}
                              onClick={() => {
                                setSelectedHostId(mountedHost.id);
                                setActiveTab("hardware");
                              }}
                              title={`点击查看硬件监控与BMC带外详情: ${mountedHost.hostname}`}
                            >
                              <div className="device-info-left">
                                <span className="device-brand-badge">{mountedHost.brand}</span>
                                <div>
                                  <div className="device-hostname">{mountedHost.hostname}</div>
                                  <div className="device-ip">IP: {mountedHost.ip} · BMC: {mountedHost.bmcIp}</div>
                                </div>
                              </div>
                              <div className="device-indicators">
                                <span className="device-u-badge">{mountedHost.uHeight}U · {mountedHost.model}</span>
                                <span className="led-indicator" title="设备在线正常运行" />
                              </div>
                            </div>
                          </div>
                        );
                      }

                      if (mountedSwitch) {
                        const isSwitchTop = slotNum === ((mountedSwitch.startU ?? 0) + (mountedSwitch.uHeight ?? 1) - 1);
                        if (!isSwitchTop) return null;
                        const heightPx = (mountedSwitch.uHeight ?? 1) * 30 - 2;

                        return (
                          <div key={slotNum} className="rack-u-row" style={{ height: heightPx }}>
                            <div className="u-num-tag">{mountedSwitch.startU ?? slotNum}U</div>
                            <div className="slot-occupied device-switch" style={{ height: "100%" }}>
                              <div className="device-info-left">
                                <span className="device-brand-badge">{mountedSwitch.brand}</span>
                                <div>
                                  <div className="device-hostname">{mountedSwitch.name}</div>
                                  <div className="device-ip">IP: {mountedSwitch.ip} · {mountedSwitch.role}</div>
                                </div>
                              </div>
                              <div className="device-indicators">
                                <span className="device-u-badge">{mountedSwitch.portCount} 口核心网设备</span>
                                <span className="led-indicator" />
                              </div>
                            </div>
                          </div>
                        );
                      }

                      // Empty U Slot
                      return (
                        <div key={slotNum} className="rack-u-row">
                          <div className="u-num-tag">{slotNum}U</div>
                          <div 
                            className="slot-empty"
                            onClick={() => handleEmptySlotClick(slotNum)}
                            title={`点击在 ${slotNum}U 空闲位快速上架新设备`}
                          >
                            <span>[ 空闲 U位 ]</span>
                            <span style={{ color: "#38bdf8" }}>＋ 上架设备</span>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>

                {/* Right Side: Cabinet Specs & Quick Telemetry */}
                <div className="rack-inspect-panel">
                  <header className="rack-meta-header">
                    <div>
                      <h4>{currentCabinet ? currentCabinet.name : "机架全景数据"}</h4>
                      <p>{currentRoom ? currentRoom.name : ""} · 物理位置：{currentCabinet?.row || "A区"}</p>
                    </div>
                    <span className="status-pill online">● 机房环境正常</span>
                  </header>

                  <div className="power-temp-strip">
                    <div className="stat-box">
                      <small>机柜负载功率 (当前 / 上限)</small>
                      <strong>{currentCabinet?.currentPower || 2.8} kW <span style={{ fontSize: 12, color: "#94a3b8" }}>/ {currentCabinet?.maxPower || 5.0} kW</span></strong>
                    </div>
                    <div className="stat-box">
                      <small>冷通道微环境温度</small>
                      <strong>{currentCabinet?.temperature || 22.5} °C</strong>
                    </div>
                  </div>

                  <div style={{ marginBottom: 16 }}>
                    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12, marginBottom: 6 }}>
                      <span>U位占用率</span>
                      <b>{currentCabinet ? Math.round((currentCabinet.usedU / 42) * 100) : 0}% ({currentCabinet?.usedU || 0} / 42 U)</b>
                    </div>
                    <div className="u-progress-bar">
                      <div 
                        className="u-progress-fill" 
                        style={{ width: `${currentCabinet ? (currentCabinet.usedU / 42) * 100 : 0}%` }}
                      />
                    </div>
                  </div>

                  {/* Device List In This Cabinet */}
                  <h5 style={{ margin: "16px 0 8px", fontSize: 13, color: "#334155" }}>
                    机柜内物理在架设备 ({cabinetHosts.length + cabinetSwitches.length} 台)
                  </h5>

                  <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                    {cabinetHosts.map(h => (
                      <div 
                        key={h.id}
                        style={{
                          display: "flex",
                          justifyContent: "space-between",
                          alignItems: "center",
                          padding: "8px 12px",
                          background: "#f8fafc",
                          borderRadius: 6,
                          border: "1px solid #e2e8f0",
                          cursor: "pointer"
                        }}
                        onClick={() => {
                          setSelectedHostId(h.id);
                          setActiveTab("hardware");
                        }}
                      >
                        <div>
                          <div style={{ fontSize: 12, fontWeight: 600, color: "#1e293b" }}>{h.hostname}</div>
                          <div style={{ fontSize: 11, color: "#64748b" }}>{h.startU ?? '?'}U-{(h.startU ?? 0) + (h.uHeight ?? 1) - 1}U · {h.brand} {h.model} · IP: {h.ip}</div>
                        </div>
                        <div style={{ display: "flex", gap: 6 }}>
                          <button 
                            className="btn-secondary"
                            style={{ padding: "3px 8px", fontSize: 11 }}
                            onClick={(e) => {
                              e.stopPropagation();
                              setSelectedHostId(h.id);
                              setActiveTab("hardware");
                            }}
                          >
                            BMC监控
                          </button>
                          <button 
                            className="btn-secondary"
                            style={{ padding: "3px 8px", fontSize: 11, color: "#ef4444" }}
                            onClick={(e) => {
                              e.stopPropagation();
                              if (confirm(`确定下架并移除服务器 [${h.hostname}] 吗？`)) {
                                onDeleteHost(h.id);
                              }
                            }}
                          >
                            下架
                          </button>
                        </div>
                      </div>
                    ))}
                    {cabinetSwitches.map(s => (
                      <div 
                        key={s.id}
                        style={{
                          display: "flex",
                          justifyContent: "space-between",
                          alignItems: "center",
                          padding: "8px 12px",
                          background: "#f0fdf4",
                          borderRadius: 6,
                          border: "1px solid #bbf7d0"
                        }}
                      >
                        <div>
                          <div style={{ fontSize: 12, fontWeight: 600, color: "#166534" }}>{s.name}</div>
                          <div style={{ fontSize: 11, color: "#4b5563" }}>{s.startU}U · {s.brand} {s.model} · {s.role}</div>
                        </div>
                        <span className="status-pill online">网络畅通</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}

            {/* VIEW 2: 2.5D ROOM FLOOR LAYOUT */}
            {activeTab === "floor" && (
              <div className="floor-grid-container">
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <div>
                    <h3 style={{ margin: 0, fontSize: 15, color: "#0f172a" }}>
                      {currentRoom ? currentRoom.name : "全网机房"} · 机柜矩阵俯瞰态势
                    </h3>
                    <p style={{ margin: "3px 0 0", fontSize: 12, color: "#64748b" }}>
                      {currentRoom?.address} · 运营商：{currentRoom?.operator} · 级别：{currentRoom?.level}
                    </p>
                  </div>
                  <span className="room-badge" style={{ fontSize: 12, padding: "4px 10px" }}>
                    双路市电 2N 容灾接入
                  </span>
                </div>

                {/* Group Cabinets by Row */}
                {["Row A", "Row B", "Row C", "Row D"].map(rowName => {
                  const rowCabs = cabinets.filter(c => 
                    (!selectedRoomId || c.roomId === selectedRoomId) && (c.row === rowName || !c.row)
                  );
                  if (rowCabs.length === 0) return null;

                  return (
                    <div key={rowName} className="floor-row-block">
                      <div className="floor-row-title">
                        <span>🏛</span> {rowName} 机柜列通道阵列
                      </div>
                      <div className="cabinet-rack-cards">
                        {rowCabs.map(cab => {
                          const isCurrent = cab.id === selectedCabinetId;
                          const cabHosts = hosts.filter(h => h.cabinetId === cab.id);
                          const usagePercent = Math.round((cab.usedU / 42) * 100);

                          return (
                            <div 
                              key={cab.id} 
                              className={`cabinet-card ${isCurrent ? "selected" : ""}`}
                              style={{ borderColor: isCurrent ? "#2563eb" : "#e2e8f0" }}
                              onClick={() => {
                                setSelectedCabinetId(cab.id);
                                setActiveTab("rack");
                              }}
                            >
                              <div className="cabinet-card-top">
                                <strong>{cab.name}</strong>
                                <span className={`status-pill ${cab.status === "正常" ? "online" : "warning"}`}>
                                  ● {cab.status}
                                </span>
                              </div>
                              <div style={{ fontSize: 11, color: "#64748b" }}>
                                编码: {cab.code} · 管理人: {cab.manager}
                              </div>
                              <div>
                                <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, marginBottom: 4 }}>
                                  <span>U位已用</span>
                                  <b>{usagePercent}% ({cab.usedU}/42U)</b>
                                </div>
                                <div className="u-progress-bar">
                                  <div className="u-progress-fill" style={{ width: `${usagePercent}%` }} />
                                </div>
                              </div>
                              <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, color: "#475569", paddingTop: 4, borderTop: "1px dashed #e2e8f0" }}>
                                <span>功耗: {cab.currentPower} kW</span>
                                <span>在架: {cabHosts.length} 台物理机</span>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}

            {/* VIEW 3: PHYSICAL ASSET TABLE */}
            {activeTab === "table" && (
              <div>
                <div className="cmdb-table-filter">
                  <div className="filter-left-inputs">
                    <input 
                      placeholder="搜索主机名 / 管理IP / 资产编号..."
                      value={tableSearch}
                      onChange={e => setTableSearch(e.target.value)}
                      style={{ width: 240 }}
                    />
                    <select value={brandFilter} onChange={e => setBrandFilter(e.target.value)}>
                      <option value="全部">全部硬件品牌</option>
                      <option value="Dell">Dell 戴尔</option>
                      <option value="华为">华为 FusionServer</option>
                      <option value="浪潮">浪潮 Inspur</option>
                    </select>
                  </div>
                  <div style={{ display: "flex", gap: 8 }}>
                    <button 
                      className="btn-primary"
                      onClick={() => {
                        setMountSlotPrefill(1);
                        setMountForm(prev => ({
                          ...prev,
                          roomId: selectedRoomId || "room-1",
                          cabinetId: selectedCabinetId || "cab-101",
                          startU: 1
                        }));
                        setSlotConflictMsg("");
                        setShowMountModal(true);
                      }}
                    >
                      ＋ 设备入柜
                    </button>
                  </div>
                </div>

                <table className="cmdb-data-table">
                  <thead>
                    <tr>
                      <th>资产编号</th>
                      <th>主机名称</th>
                      <th>管理 IP</th>
                      <th>带外 BMC IP</th>
                      <th>机房 / 机柜</th>
                      <th>U位空间</th>
                      <th>硬件品牌与型号</th>
                      <th>计算配置 (CPU/内存)</th>
                      <th>业务归属</th>
                      <th>状态</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredTableHosts.map(h => {
                      const room = rooms.find(r => r.id === h.roomId);
                      const cab = cabinets.find(c => c.id === h.cabinetId);
                      const biz = businesses.find(b => b.id === h.businessId);

                      return (
                        <tr key={h.id}>
                          <td style={{ fontFamily: "monospace", fontWeight: 600 }}>{h.assetNo}</td>
                          <td style={{ fontWeight: 600, color: "#1e293b" }}>{h.hostname}</td>
                          <td style={{ fontFamily: "monospace", color: "#2563eb" }}>{h.ip}</td>
                          <td style={{ fontFamily: "monospace", color: "#64748b" }}>{h.bmcIp}</td>
                          <td>{room?.city} · {cab?.name}</td>
                          <td><span className="room-badge">{h.startU ?? '?'}U - {(h.startU ?? 0) + (h.uHeight ?? 1) - 1}U ({h.uHeight ?? '?'}U)</span></td>
                          <td>{h.brand} {h.model}</td>
                          <td style={{ fontSize: 11, color: "#64748b" }}>{h.cpu} · {h.memory}</td>
                          <td>
                            <button 
                              style={{ background: "none", border: 0, color: "#0284c7", cursor: "pointer", textDecoration: "underline" }}
                              onClick={() => onNavigateToBusiness?.(h.businessId)}
                            >
                              {biz?.name || "核心生产"}
                            </button>
                          </td>
                          <td><span className="status-pill online">● 在线运行</span></td>
                          <td>
                            <button 
                              className="btn-secondary" 
                              style={{ padding: "3px 7px", fontSize: 11, marginRight: 6 }}
                              onClick={() => {
                                setSelectedHostId(h.id);
                                setActiveTab("hardware");
                              }}
                            >
                              硬件监控
                            </button>
                            <button 
                              className="btn-secondary" 
                              style={{ padding: "3px 7px", fontSize: 11, color: "#ef4444" }}
                              onClick={() => {
                                if (confirm(`确认下架服务器 ${h.hostname}？`)) {
                                  onDeleteHost(h.id);
                                }
                              }}
                            >
                              下架
                            </button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}

            {/* VIEW 4: HARDWARE SENSOR & BMC MONITOR */}
            {activeTab === "hardware" && (
              <div style={{ display: "grid", gridTemplateColumns: "320px 1fr", gap: 16 }}>
                {/* Left Host Selector */}
                <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 12 }}>
                  <h4 style={{ margin: "0 0 10px", fontSize: 14 }}>已纳管物理主机</h4>
                  <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                    {hosts.map(h => (
                      <div 
                        key={h.id}
                        style={{
                          padding: "8px 10px",
                          borderRadius: 6,
                          border: h.id === selectedHostId ? "1px solid #3b82f6" : "1px solid #e2e8f0",
                          background: h.id === selectedHostId ? "#eff6ff" : "#fff",
                          cursor: "pointer"
                        }}
                        onClick={() => setSelectedHostId(h.id)}
                      >
                        <div style={{ fontSize: 12, fontWeight: 600, color: "#0f172a" }}>{h.hostname}</div>
                        <div style={{ fontSize: 11, color: "#64748b" }}>IP: {h.ip} · BMC: {h.bmcIp}</div>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Right Hardware Telemetry Dash */}
                {selectedHost ? (
                  <div className="hardware-card">
                    <h5>
                      <span>⚡ {selectedHost.hostname} 物理服务器底层硬件监控</span>
                      <span className="status-pill online">iDRAC/iBMC 连接畅通</span>
                    </h5>

                    <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 10, marginBottom: 14 }}>
                      <div className="sensor-tile">
                        <span>CPU 核心温度</span>
                        <b style={{ color: selectedHost.cpuTemp > 50 ? "#f59e0b" : "#38bdf8" }}>{selectedHost.cpuTemp} °C</b>
                      </div>
                      <div className="sensor-tile">
                        <span>主板整体温度</span>
                        <b>{selectedHost.boardTemp} °C</b>
                      </div>
                      <div className="sensor-tile">
                        <span>机箱进风口温度</span>
                        <b>{selectedHost.inletTemp} °C</b>
                      </div>
                      <div className="sensor-tile">
                        <span>实时整机功率</span>
                        <b style={{ color: "#22c55e" }}>{selectedHost.powerWatts} W</b>
                      </div>
                    </div>

                    <div style={{ marginBottom: 14 }}>
                      <span style={{ fontSize: 12, color: "#94a3b8", display: "block", marginBottom: 6 }}>双路冗余电源 (1+1 Redundant PSU)</span>
                      <div className="psu-status-row">
                        <span>电源模块 1 (PSU-01)</span>
                        <b style={{ color: "#22c55e" }}>{selectedHost.psu1Status}</b>
                      </div>
                      <div className="psu-status-row">
                        <span>电源模块 2 (PSU-02)</span>
                        <b style={{ color: "#22c55e" }}>{selectedHost.psu2Status}</b>
                      </div>
                    </div>

                    <div style={{ marginBottom: 14 }}>
                      <span style={{ fontSize: 12, color: "#94a3b8", display: "block", marginBottom: 6 }}>风扇转速与风道状态</span>
                      <div className="psu-status-row">
                        <span>四通道脉宽调制风扇模块</span>
                        <b>{selectedHost.fanRpm}</b>
                      </div>
                    </div>

                    <div style={{ marginBottom: 14 }}>
                      <span style={{ fontSize: 12, color: "#94a3b8", display: "block", marginBottom: 6 }}>物理存储阵列与磁盘控制器</span>
                      <div className="psu-status-row">
                        <span>LSI / PERC 硬件RAID卡状态</span>
                        <b style={{ color: "#22c55e" }}>{selectedHost.raidStatus}</b>
                      </div>
                    </div>

                    <div style={{ display: "flex", gap: 10, marginTop: 16, paddingTop: 14, borderTop: "1px solid #334155" }}>
                      <button 
                        className="btn-primary"
                        onClick={() => alert(`已向带外管理卡 ${selectedHost.bmcIp} 发起 HTML5 KVM 远程桌面控制台连接请求！`)}
                      >
                        打开远程 KVM 虚拟控制台
                      </button>
                      <button 
                        className="btn-secondary"
                        onClick={() => alert(`向 Agent [${selectedHost.hostname}] 发起底层硬件自检诊断（IPMI SEL 日志扫描）已完成，未发现致命告警！`)}
                      >
                        运行硬件诊断扫描
                      </button>
                    </div>
                  </div>
                ) : (
                  <div>未选择物理机</div>
                )}
              </div>
            )}
          </div>
        </main>
      </div>

      {/* MODAL 1: NEW ROOM */}
      {showRoomModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleRoomSubmit}>
            <div className="cmdb-modal-header">
              <h3>🏢 新建 IDC 数据中心机房</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowRoomModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body">
              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 机房名称</label>
                  <input 
                    placeholder="如：北京海淀自建机房" 
                    value={roomForm.name} 
                    onChange={e => setRoomForm({ ...roomForm, name: e.target.value })}
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 机房编码</label>
                  <input 
                    placeholder="如：BJ-HD-01" 
                    value={roomForm.code} 
                    onChange={e => setRoomForm({ ...roomForm, code: e.target.value })}
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>运营商专线</label>
                  <select 
                    value={roomForm.operator} 
                    onChange={e => setRoomForm({ ...roomForm, operator: e.target.value as any })}
                  >
                    <option value="BGP多线">BGP多线 (电信+联通+移动)</option>
                    <option value="中国电信">中国电信</option>
                    <option value="中国联通">中国联通</option>
                    <option value="中国移动">中国移动</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>机房安全建设级别</label>
                  <select 
                    value={roomForm.level} 
                    onChange={e => setRoomForm({ ...roomForm, level: e.target.value as any })}
                  >
                    <option value="T3+">Tier 3+ (高可用容灾)</option>
                    <option value="T3">Tier 3</option>
                    <option value="T4">Tier 4 (容错级顶级架构)</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>所在城市</label>
                  <input 
                    value={roomForm.city} 
                    onChange={e => setRoomForm({ ...roomForm, city: e.target.value })} 
                  />
                </div>
                <div className="form-field-item">
                  <label>现场负责人及电话</label>
                  <input 
                    placeholder="张工 138xxxx" 
                    value={roomForm.contact} 
                    onChange={e => setRoomForm({ ...roomForm, contact: e.target.value })} 
                  />
                </div>
              </div>

              <div className="form-field-item">
                <label>详细地理位置 / 机房楼栋</label>
                <input 
                  placeholder="如：科技园区A栋3层核心机房" 
                  value={roomForm.address} 
                  onChange={e => setRoomForm({ ...roomForm, address: e.target.value })} 
                />
              </div>

              <div className="form-field-item">
                <label>备注说明</label>
                <textarea 
                  placeholder="输入机房配套设施等说明..." 
                  value={roomForm.remark} 
                  onChange={e => setRoomForm({ ...roomForm, remark: e.target.value })} 
                />
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowRoomModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">确 定 创 建</button>
            </div>
          </form>
        </div>
      )}

      {/* MODAL 2: NEW CABINET */}
      {showCabinetModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleCabinetSubmit}>
            <div className="cmdb-modal-header">
              <h3>▥ 新建标准机架 (Cabinet)</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowCabinetModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body">
              <div className="form-field-item">
                <label>* 所属机房</label>
                <select 
                  value={cabinetForm.roomId} 
                  onChange={e => setCabinetForm({ ...cabinetForm, roomId: e.target.value })}
                >
                  {rooms.map(r => (
                    <option key={r.id} value={r.id}>{r.name} ({r.city})</option>
                  ))}
                </select>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 机柜名称 / 编号</label>
                  <input 
                    placeholder="如：A03 机柜" 
                    value={cabinetForm.name} 
                    onChange={e => setCabinetForm({ ...cabinetForm, name: e.target.value })}
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>排列通道 (Row)</label>
                  <select 
                    value={cabinetForm.row} 
                    onChange={e => setCabinetForm({ ...cabinetForm, row: e.target.value })}
                  >
                    <option value="Row A">Row A (A区冷通道)</option>
                    <option value="Row B">Row B (B区冷通道)</option>
                    <option value="Row C">Row C (C区网络柜列)</option>
                    <option value="Row D">Row D (D区边缘AI柜列)</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>额定承载最大功耗 (kW)</label>
                  <input 
                    type="number" 
                    step="0.5" 
                    value={cabinetForm.maxPower} 
                    onChange={e => setCabinetForm({ ...cabinetForm, maxPower: parseFloat(e.target.value) || 5.0 })}
                  />
                </div>
                <div className="form-field-item">
                  <label>责任运维人员</label>
                  <input 
                    placeholder="如：王工" 
                    value={cabinetForm.manager} 
                    onChange={e => setCabinetForm({ ...cabinetForm, manager: e.target.value })}
                  />
                </div>
              </div>

              <div className="form-field-item">
                <label>用途说明</label>
                <textarea 
                  placeholder="如：承载集团数据中心存储设备..." 
                  value={cabinetForm.remark} 
                  onChange={e => setCabinetForm({ ...cabinetForm, remark: e.target.value })}
                />
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowCabinetModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">创 建 机 柜</button>
            </div>
          </form>
        </div>
      )}

      {/* MODAL 3: MOUNT HOST ON RACK */}
      {showMountModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleMountSubmit}>
            <div className="cmdb-modal-header">
              <h3>▣ 物理服务器入柜与硬件上架</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowMountModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body">
              {slotConflictMsg && (
                <div style={{ background: "#fef2f2", border: "1px solid #fca5a5", color: "#b91c1c", padding: "8px 12px", borderRadius: 6, fontSize: 12 }}>
                  ⚠ {slotConflictMsg}
                </div>
              )}

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 所属机房</label>
                  <select 
                    value={mountForm.roomId} 
                    onChange={e => {
                      const newRId = e.target.value;
                      const roomCabs = cabinets.filter(c => c.roomId === newRId);
                      setMountForm({ ...mountForm, roomId: newRId, cabinetId: roomCabs[0]?.id || "" });
                      setSlotConflictMsg("");
                    }}
                  >
                    {rooms.map(r => (
                      <option key={r.id} value={r.id}>{r.name}</option>
                    ))}
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 所属机柜</label>
                  <select 
                    value={mountForm.cabinetId} 
                    onChange={e => {
                      setMountForm({ ...mountForm, cabinetId: e.target.value });
                      setSlotConflictMsg("");
                    }}
                  >
                    {cabinets.filter(c => c.roomId === mountForm.roomId).map(c => (
                      <option key={c.id} value={c.id}>{c.name} ({c.row})</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 起始 U 位 (1 ~ 42)</label>
                  <input 
                    type="number" 
                    min="1" 
                    max="42" 
                    value={mountForm.startU} 
                    onChange={e => {
                      setMountForm({ ...mountForm, startU: parseInt(e.target.value) || 1 });
                      setSlotConflictMsg("");
                    }}
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 占用高度 (U数)</label>
                  <select 
                    value={mountForm.uHeight} 
                    onChange={e => {
                      setMountForm({ ...mountForm, uHeight: parseInt(e.target.value) || 1 });
                      setSlotConflictMsg("");
                    }}
                  >
                    <option value={1}>1U (高密度刀片/接入网)</option>
                    <option value={2}>2U (标准双路服务器)</option>
                    <option value={4}>4U (多GPU智算/高密存储)</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 主机名 (Hostname)</label>
                  <input 
                    placeholder="如：bj-prod-api01" 
                    value={mountForm.hostname} 
                    onChange={e => setMountForm({ ...mountForm, hostname: e.target.value })}
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 资产编号 (Asset No)</label>
                  <input 
                    placeholder="如：SRV-BJ-009" 
                    value={mountForm.assetNo} 
                    onChange={e => setMountForm({ ...mountForm, assetNo: e.target.value })}
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 内网业务管理 IP</label>
                  <input 
                    placeholder="如：10.100.10.55" 
                    value={mountForm.ip} 
                    onChange={e => setMountForm({ ...mountForm, ip: e.target.value })}
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 带外远程管理卡 BMC IP</label>
                  <input 
                    placeholder="如：192.168.100.55" 
                    value={mountForm.bmcIp} 
                    onChange={e => setMountForm({ ...mountForm, bmcIp: e.target.value })}
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>硬件厂商品牌</label>
                  <select 
                    value={mountForm.brand} 
                    onChange={e => setMountForm({ ...mountForm, brand: e.target.value as any })}
                  >
                    <option value="Dell">Dell 戴尔</option>
                    <option value="华为">华为 FusionServer</option>
                    <option value="浪潮">浪潮 Inspur</option>
                    <option value="H3C">新华三 H3C</option>
                    <option value="联想">联想 ThinkSystem</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>服务器型号</label>
                  <input 
                    value={mountForm.model} 
                    onChange={e => setMountForm({ ...mountForm, model: e.target.value })}
                  />
                </div>
              </div>

              <div className="form-field-item">
                <label>所属业务系统模型</label>
                <select 
                  value={mountForm.businessId} 
                  onChange={e => setMountForm({ ...mountForm, businessId: e.target.value })}
                >
                  {businesses.map(b => (
                    <option key={b.id} value={b.id}>{b.name} ({b.level})</option>
                  ))}
                </select>
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowMountModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">确 认 提 交 上 架</button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
