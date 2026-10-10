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

// Read v363/v361 file to test exact record counts
test("v363 Excel workbook contains all sheets with exact record counts (301 hardware, 47 db rows, 81 ops)", () => {
  const isV363 = fs.existsSync("信息资产台账-v363.xlsx");
  const filePath = isV363 ? "信息资产台账-v363.xlsx" : (fs.existsSync("信息资产台账-v361.xlsx") ? "信息资产台账-v361.xlsx" : "信息资产台账-v360.xlsx");
  const buf = fs.readFileSync(filePath);
  const wb = XLSX.read(buf, { type: "buffer" });
  
  const expectedSheets = ["00-代码表", "01-系统目录", "02-硬件设备", "03-数据库", "04-中间件", "05-备份", "06-运维账号"];
  for (const sheet of ["02-硬件设备", "03-数据库", "04-中间件", "05-备份", "06-运维账号"]) {
    assert.ok(wb.SheetNames.includes(sheet), `Missing expected sheet: ${sheet}`);
  }

  // 02-硬件设备: Header is row 4, v363 has 301 data items, v361 has 299 data items
  const hwSheet = wb.Sheets["02-硬件设备"];
  const hwRows = XLSX.utils.sheet_to_json(hwSheet, { header: 1, defval: null });
  const hwDataRows = hwRows.slice(4).filter(r => r && r.some(c => c !== null && String(c).trim() !== ""));
  const expectedHwCount = isV363 ? 301 : 299;
  assert.equal(hwDataRows.length, expectedHwCount, `Hardware data count should be exactly ${expectedHwCount}, got ${hwDataRows.length}`);

  // 03-数据库: Header is row 4, rows 5..51 are actual rows (47 items including sequence templates, 44 valid)
  const dbSheet = wb.Sheets["03-数据库"];
  const dbRows = XLSX.utils.sheet_to_json(dbSheet, { header: 1, defval: null });
  const dbDataRows = dbRows.slice(4).filter(r => r && r.some(c => c !== null && String(c).trim() !== ""));
  assert.equal(dbDataRows.length, 47, `Database data count should be 47, got ${dbDataRows.length}`);

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

test("MySQL schema and v363 initialization SQL files exist and contain valid statements", () => {
  assert.ok(fs.existsSync("db/mysql_schema.sql"), "db/mysql_schema.sql must exist");
  assert.ok(fs.existsSync("db/init_mysql_v363.sql"), "db/init_mysql_v363.sql must exist");
  assert.ok(fs.existsSync("db/init_mysql_v361.sql"), "db/init_mysql_v361.sql must exist");

  const schemaContent = fs.readFileSync("db/mysql_schema.sql", "utf-8");
  assert.ok(schemaContent.includes("cmdb_hardware_assets"));
  assert.ok(schemaContent.includes("cmdb_databases"));
  assert.ok(schemaContent.includes("cmdb_ops_accounts"));

  const initContent = fs.readFileSync("db/init_mysql_v363.sql", "utf-8");
  assert.ok(initContent.includes("INSERT INTO `cmdb_hardware_assets`"));
  assert.ok(initContent.includes("INSERT INTO `cmdb_databases`"));
  assert.ok(initContent.includes("INSERT INTO `cmdb_ops_accounts`"));
});
