import { fetchAllCmdbData } from "@/lib/mysqlService";

export async function GET() {
  try {
    const data = await fetchAllCmdbData();
    return Response.json(data, {
      status: 200,
      headers: {
        "Cache-Control": "no-store, max-age=0"
      }
    });
  } catch (error: any) {
    return Response.json(
      {
        ok: false,
        error: error?.message || "Failed to fetch CMDB records"
      },
      { status: 500 }
    );
  }
}
