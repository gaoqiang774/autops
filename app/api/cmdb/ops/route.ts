import { fetchAllCmdbData } from "@/lib/mysqlService";

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url);
    const projectId = searchParams.get("projectId");
    const projectName = searchParams.get("projectName");
    const customerName = searchParams.get("customerName");
    const keyword = searchParams.get("keyword") || searchParams.get("q");

    const data = await fetchAllCmdbData();
    let list = data.opsRecords;

    if (projectId && projectId !== "all") {
      list = list.filter(o => o.projectId === projectId || o.projectName === projectId);
    }
    if (projectName && projectName !== "all" && projectName !== "全部") {
      const p = projectName.toLowerCase();
      list = list.filter(o => o.projectName && o.projectName.toLowerCase().includes(p));
    }
    if (customerName && customerName !== "all" && customerName !== "全部") {
      const c = customerName.toLowerCase();
      list = list.filter(o => o.customerName && o.customerName.toLowerCase().includes(c));
    }
    if (keyword) {
      const k = keyword.toLowerCase();
      list = list.filter(o => 
        (o.projectName && o.projectName.toLowerCase().includes(k)) ||
        (o.customerName && o.customerName.toLowerCase().includes(k)) ||
        (o.vpnUserName && o.vpnUserName.toLowerCase().includes(k)) ||
        (o.bastionUserName && o.bastionUserName.toLowerCase().includes(k))
      );
    }

    return Response.json({
      ok: true,
      source: data.source,
      total: list.length,
      records: list
    });
  } catch (error: any) {
    return Response.json({ ok: false, error: error?.message }, { status: 500 });
  }
}
