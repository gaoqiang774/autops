export async function POST(request: Request) {
  const { account, password } = (await request.json()) as { account?: string; password?: string };
  const userAccount = (account || "").trim();
  
  // Allow admin and configured users with password 123456 or non-empty password
  if (!userAccount || (password !== "123456" && password !== "admin123" && password !== "Ops@2026")) {
    return Response.json({ message: "账号或密码错误 (默认密码为 123456)" }, { status: 401 });
  }

  return new Response(
    JSON.stringify({
      ok: true,
      user: {
        account: userAccount,
        name: userAccount === "admin" ? "系统管理员" : userAccount
      }
    }),
    {
      headers: {
        "content-type": "application/json",
        "set-cookie": `autoops_session=${userAccount}; Path=/; HttpOnly; SameSite=Lax; Max-Age=86400`
      }
    }
  );
}
