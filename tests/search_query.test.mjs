import test from "node:test";
import assert from "node:assert/strict";
import { initialProjects, initialPhysicalHosts, initialVms, initialDatabases, initialMiddlewares, initialBackups, initialOpsRecords } from "../app/cmdbData.ts";

test("Customer and project name search queries over CMDB data", () => {
  const allAssets = [...initialPhysicalHosts, ...initialVms];

  // Test Customer query on Assets
  const bjrsAssets = allAssets.filter(a => a.customerName === "北京市人力资源和社会保障局");
  assert.ok(bjrsAssets.length > 0, "Should find assets for 北京市人力资源和社会保障局");

  // Test Project query on Assets
  const sanxianAssets = allAssets.filter(a => a.projectName === "原三险系统");
  assert.ok(sanxianAssets.length > 0, "Should find assets for 原三险系统");
  assert.equal(sanxianAssets.every(a => a.projectName === "原三险系统"), true);

  // Test Customer query on Databases
  const bjrsDbs = initialDatabases.filter(d => d.customerName === "北京市人力资源和社会保障局");
  assert.ok(bjrsDbs.length > 0, "Should find databases for 北京市人力资源和社会保障局");

  // Test Project query on Databases
  const sanxianDbs = initialDatabases.filter(d => d.projectName === "原三险系统");
  assert.ok(sanxianDbs.length > 0, "Should find databases for 原三险系统");

  // Test Customer query on Ops records
  const unionOps = initialOpsRecords.filter(o => o.customerName === "北京市职工互助保障服务中心");
  assert.ok(unionOps.length > 0, "Should find ops records for 北京市职工互助保障服务中心");

  // Test Project query on Ops records
  const huzhuOps = initialOpsRecords.filter(o => o.projectName === "工会互助保险信息系统");
  assert.ok(huzhuOps.length > 0, "Should find ops records for 工会互助保险信息系统");
  assert.equal(huzhuOps.every(o => o.projectName === "工会互助保险信息系统"), true);

  // Combined Customer + Project query
  const combined = allAssets.filter(a => a.customerName === "北京市人力资源和社会保障局" && a.projectName === "原三险系统");
  assert.ok(combined.length > 0, "Should find assets matching both customer and project");
});
