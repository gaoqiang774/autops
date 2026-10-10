package service

import (
	"dodevops-api/api/cmdb/model"
	"testing"
)

func TestBuildAssetVOListIncludesVmCount(t *testing.T) {
	svc := &CmdbIdcAssetServiceImpl{}
	list := []model.CmdbIdcAsset{
		{
			ID:              127,
			RoomID:          1,
			Room:            model.CmdbIdcRoom{Name: "亚森机房"},
			CabinetID:       13,
			Cabinet:         model.CmdbIdcCabinet{Name: "02列08架"},
			CabinetPosition: 0,
			IP:              "192.168.34.12",
		},
		{
			ID:              128,
			RoomID:          1,
			Room:            model.CmdbIdcRoom{Name: "亚森机房"},
			CabinetID:       14,
			Cabinet:         model.CmdbIdcCabinet{Name: "02列09架"},
			CabinetPosition: 10,
			IP:              "192.168.34.13",
		},
	}

	vos := svc.buildAssetVOList(list, map[uint]int64{
		127: 3,
	})

	if len(vos) != 2 {
		t.Fatalf("buildAssetVOList len = %d, want 2", len(vos))
	}
	if vos[0].VmCount != 3 {
		t.Fatalf("first asset vmCount = %d, want 3", vos[0].VmCount)
	}
	if vos[1].VmCount != 0 {
		t.Fatalf("second asset vmCount = %d, want 0", vos[1].VmCount)
	}
	if vos[0].RoomName != "亚森机房" || vos[0].CabinetName != "02列08架" {
		t.Fatalf("first asset names = (%q, %q), want (%q, %q)", vos[0].RoomName, vos[0].CabinetName, "亚森机房", "02列08架")
	}
}

func TestCollectAssetIDsSkipsZeroID(t *testing.T) {
	ids := collectAssetIDs([]model.CmdbIdcAsset{
		{ID: 127},
		{ID: 0},
		{ID: 129},
	})

	if len(ids) != 2 {
		t.Fatalf("collectAssetIDs len = %d, want 2", len(ids))
	}
	if ids[0] != 127 || ids[1] != 129 {
		t.Fatalf("collectAssetIDs = %v, want [127 129]", ids)
	}
}

func TestParseCabinetNamePosition(t *testing.T) {
	tests := []struct {
		name          string
		wantRowNo     int
		wantCabinetNo int
		wantMatched   bool
	}{
		{name: "1列 04柜", wantRowNo: 1, wantCabinetNo: 4, wantMatched: true},
		{name: "01列05柜", wantRowNo: 1, wantCabinetNo: 5, wantMatched: true},
		{name: "02列09架", wantRowNo: 2, wantCabinetNo: 9, wantMatched: true},
		{name: "测试柜A", wantMatched: false},
	}

	for _, tt := range tests {
		rowNo, cabinetNo, matched := parseCabinetNamePosition(tt.name)
		if rowNo != tt.wantRowNo || cabinetNo != tt.wantCabinetNo || matched != tt.wantMatched {
			t.Fatalf("parseCabinetNamePosition(%q) = (%d, %d, %v), want (%d, %d, %v)",
				tt.name, rowNo, cabinetNo, matched, tt.wantRowNo, tt.wantCabinetNo, tt.wantMatched)
		}
	}
}

func TestBuildRoomLayoutOrdersCabinetsByParsedName(t *testing.T) {
	room := model.CmdbIdcRoom{
		ID:   1,
		Name: "亚森机房",
		Cabinets: []model.CmdbIdcCabinet{
			{ID: 10, RoomID: 1, Name: "01列10柜", TotalU: 42},
			{ID: 4, RoomID: 1, Name: "1列 04柜", TotalU: 42},
			{ID: 5, RoomID: 1, Name: "01列05柜", TotalU: 42},
			{ID: 21, RoomID: 1, Name: "02列01柜", TotalU: 42},
			{ID: 99, RoomID: 1, Name: "备用柜A", TotalU: 42},
		},
	}
	assets := []model.CmdbIdcAsset{
		{ID: 1, RoomID: 1, CabinetID: 4, CabinetPosition: 1, CabinetUSize: 2},
		{ID: 2, RoomID: 1, CabinetID: 5, CabinetPosition: 3, CabinetUSize: 1},
	}

	layout := buildRoomLayout(room, assets)

	if layout.LayoutMode != "name_order" {
		t.Fatalf("layoutMode = %q, want name_order", layout.LayoutMode)
	}
	if layout.CabinetCount != 5 || layout.AssetCount != 2 {
		t.Fatalf("counts = (%d, %d), want (5, 2)", layout.CabinetCount, layout.AssetCount)
	}
	if len(layout.Rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(layout.Rows))
	}

	firstRow := layout.Rows[0]
	if firstRow.RowNo != 1 || firstRow.Y != 0 {
		t.Fatalf("first row = (%d, %d), want (1, 0)", firstRow.RowNo, firstRow.Y)
	}
	gotNames := []string{firstRow.Cabinets[0].Name, firstRow.Cabinets[1].Name, firstRow.Cabinets[2].Name}
	wantNames := []string{"1列 04柜", "01列05柜", "01列10柜"}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("first row names = %v, want %v", gotNames, wantNames)
		}
		if firstRow.Cabinets[i].X != i {
			t.Fatalf("cabinet %q x = %d, want %d", firstRow.Cabinets[i].Name, firstRow.Cabinets[i].X, i)
		}
	}
	if firstRow.Cabinets[0].UsedU != 2 || firstRow.Cabinets[0].AssetCount != 1 {
		t.Fatalf("first cabinet usedU/assetCount = (%d, %d), want (2, 1)",
			firstRow.Cabinets[0].UsedU, firstRow.Cabinets[0].AssetCount)
	}
	if len(layout.UnmatchedCabinets) != 1 || layout.UnmatchedCabinets[0].Name != "备用柜A" {
		t.Fatalf("unmatched = %+v, want 备用柜A", layout.UnmatchedCabinets)
	}
}
