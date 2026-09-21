import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import * as XLSX from "xlsx";

// Read original v351 file to test sheet detection and data structure
test("v351 Excel workbook contains all 5 asset sheets and can be read by xlsx", () => {
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
});
