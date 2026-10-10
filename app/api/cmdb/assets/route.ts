import { fetchAllCmdbData } from "@/lib/mysqlService";

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url);
    const projectId = searchParams.get("projectId");
    const type = searchParams.get("type"); // physical | vm
    const projectName = searchParams.get("projectName");
    const customerName = searchParams.get("customerName");
    const ip = searchParams.get("ip");
    const keyword = searchParams.get("keyword") || searchParams.get("q");

    const data = await fetchAllCmdbData();
    let list = [...data.physicalHosts, ...data.vms];

    if (projectId && projectId !== "all") {
      list = list.filter(a => a.projectId === projectId || a.projectName === projectId);
    }
    if (projectName && projectName !== "all" && projectName !== "全部") {
      const p = projectName.toLowerCase();
      list = list.filter(a => a.projectName && a.projectName.toLowerCase().includes(p));
    }
    if (customerName && customerName !== "all" && customerName !== "全部") {
      const c = customerName.toLowerCase();
      list = list.filter(a => a.customerName && a.customerName.toLowerCase().includes(c));
    }
    if (ip) {
      const targetIp = ip.toLowerCase();
      list = list.filter(a => 
        (a.privateIp && a.privateIp.toLowerCase().includes(targetIp)) ||
        ((a as any).ip && (a as any).ip.toLowerCase().includes(targetIp)) ||
        (a.internalWanIp && a.internalWanIp.toLowerCase().includes(targetIp)) ||
        (a.vip && a.vip.toLowerCase().includes(targetIp)) ||
        (a.eip && a.eip.toLowerCase().includes(targetIp))
      );
    }
    if (keyword) {
      const k = keyword.toLowerCase();
      list = list.filter(a => 
        (a.name && a.name.toLowerCase().includes(k)) ||
        (a.projectName && a.projectName.toLowerCase().includes(k)) ||
        (a.customerName && a.customerName.toLowerCase().includes(k)) ||
        (a.privateIp && a.privateIp.toLowerCase().includes(k)) ||
        ((a as any).ip && (a as any).ip.toLowerCase().includes(k))
      );
    }
    if (type === "physical") {
      list = list.filter(a => a.deviceType === "物理机");
    } else if (type === "vm") {
      list = list.filter(a => a.deviceType !== "物理机");
    }

    return Response.json({
      ok: true,
      source: data.source,
      total: list.length,
      assets: list
    });
  } catch (error: any) {
    return Response.json({ ok: false, error: error?.message }, { status: 500 });
  }
}
