import { fetchAllCmdbData } from "@/lib/mysqlService";

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url);
    const projectId = searchParams.get("projectId");

    const data = await fetchAllCmdbData();
    let list = data.opsRecords;

    if (projectId && projectId !== "all") {
      list = list.filter(o => o.projectId === projectId || o.projectName === projectId);
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
