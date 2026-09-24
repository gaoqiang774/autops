import { fetchAllCmdbData } from "@/lib/mysqlService";

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url);
    const projectId = searchParams.get("projectId");
    const type = searchParams.get("type"); // physical | vm

    const data = await fetchAllCmdbData();
    let list = [...data.physicalHosts, ...data.vms];

    if (projectId && projectId !== "all") {
      list = list.filter(a => a.projectId === projectId || a.projectName === projectId);
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
