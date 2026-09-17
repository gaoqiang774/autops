import type { Metadata } from "next";
import "./globals.css";
import "./extra.css";
import "./cmdb.css";

export const metadata: Metadata = {
  title: "北控伟仕智能运维平台",
  description: "北控伟仕智能运维平台",
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
