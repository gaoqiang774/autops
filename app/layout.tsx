import type { Metadata } from "next";
import "./globals.css";
import "./extra.css";
import "./cmdb.css";

export const metadata: Metadata = {
  title: "运维信息资产管理平台",
  description: "运维信息资产管理平台",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN">
      <body>{children}</body>
    </html>
  );
}
