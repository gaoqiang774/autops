import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import * as XLSX from "xlsx";

// Read v351 file if present (legacy compatibility)
test("v351 Excel workbook structure test if exists", () => {
  if (fs.existsSync("信息资产台账-v351.xlsx")) {
    const buf = fs.readFileSync("信息资产台账-v351.xlsx");
    const wb = XLSX.read(buf, { type: "buffer" });
    
    const expectedSheets = ["02-硬件设备", "03-数据库", "04-中间件", "05-备份", "06-运维"];
    for (const sheet of expectedSheets) {
      assert.ok(wb.SheetNames.includes(sheet), `Missing expected sheet: ${sheet}`);
      const ws = wb.Sheets[sheet];
      assert.ok(ws, `Sheet ${sheet} is empty or unreadable`);
      const rows = XLSX.utils.sheet_to_json(ws, { header: 1, defval: null });
      assert.ok(rows.length > 3, `Sheet ${sheet} has insufficient rows: ${rows.length}`);
    }
  }
});

// Read v360 file to test exact 299 hardware records and all 5 sheets
test("v360 Excel workbook contains all sheets with exact record counts (299 hardware, 47 db, 81 ops)", () => {
  const buf = fs.readFileSync("信息资产台账-v360.xlsx");
  const wb = XLSX.read(buf, { type: "buffer" });
  
  const expectedSheets = ["02-硬件设备", "03-数据库", "04-中间件", "05-备份", "06-运维账号"];
  for (const sheet of expectedSheets) {
    assert.ok(wb.SheetNames.includes(sheet), `Missing expected sheet: ${sheet}`);
  }

  // 02-硬件设备: Header is row 4, rows 5..303 are actual data (299 items)
  const hwSheet = wb.Sheets["02-硬件设备"];
  const hwRows = XLSX.utils.sheet_to_json(hwSheet, { header: 1, defval: null });
  const hwDataRows = hwRows.slice(4).filter(r => r && r.some(c => c !== null && String(c).trim() !== ""));
  assert.equal(hwDataRows.length, 299, `Hardware data count should be exactly 299, got ${hwDataRows.length}`);

  // 03-数据库: Header is row 4, rows 5..51 are actual data (47 items)
  const dbSheet = wb.Sheets["03-数据库"];
  const dbRows = XLSX.utils.sheet_to_json(dbSheet, { header: 1, defval: null });
  const dbDataRows = dbRows.slice(4).filter(r => r && r.some(c => c !== null && String(c).trim() !== ""));
  assert.equal(dbDataRows.length, 47, `Database data count should be exactly 47, got ${dbDataRows.length}`);

  // 06-运维账号: Header is row 3, rows 4..84 are actual data (81 items)
  const opsSheet = wb.Sheets["06-运维账号"];
  const opsRows = XLSX.utils.sheet_to_json(opsSheet, { header: 1, defval: null });
  const opsDataRows = opsRows.slice(3).filter(r => r && r.some(c => c !== null && String(c).trim() !== ""));
  assert.equal(opsDataRows.length, 81, `Ops account count should be exactly 81, got ${opsDataRows.length}`);

  // Verify that VPN accounts have associated user names (column index 10: '对应名称')
  const userNames = opsDataRows.map(r => r[10]).filter(Boolean);
  assert.ok(userNames.length > 50, `At least 50 ops records should have VPN user names, got ${userNames.length}`);
  assert.ok(userNames.includes("翟焕净"), "Should contain sample VPN user name 翟焕净");
  assert.ok(userNames.includes("宋佳艺"), "Should contain sample VPN user name 宋佳艺");
});

test("MySQL schema and v360 initialization SQL files exist and contain valid statements", () => {
  assert.ok(fs.existsSync("db/mysql_schema.sql"), "db/mysql_schema.sql must exist");
  assert.ok(fs.existsSync("db/init_mysql_v360.sql"), "db/init_mysql_v360.sql must exist");

  const schemaContent = fs.readFileSync("db/mysql_schema.sql", "utf-8");
  assert.ok(schemaContent.includes("cmdb_hardware_assets"));
  assert.ok(schemaContent.includes("cmdb_databases"));
  assert.ok(schemaContent.includes("cmdb_ops_accounts"));

  const initContent = fs.readFileSync("db/init_mysql_v360.sql", "utf-8");
  assert.ok(initContent.includes("INSERT INTO `cmdb_hardware_assets`"));
  assert.ok(initContent.includes("INSERT INTO `cmdb_databases`"));
  assert.ok(initContent.includes("INSERT INTO `cmdb_ops_accounts`"));
});
