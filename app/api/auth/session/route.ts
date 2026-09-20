export async function GET(request: Request) {
  const cookie = request.headers.get("cookie") || "";
  const match = cookie.match(/autoops_session=([^;]+)/);
  const account = match ? match[1] : null;
  const ok = !!account;
  return Response.json(
    {
      ok,
      user: ok
        ? {
            account: account,
            name: account === "admin" ? "系统管理员" : account
          }
        : null
    },
    { status: ok ? 200 : 401 }
  );
}
