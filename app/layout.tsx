import type { Metadata } from "next";
import "./globals.css";
import "./extra.css";

export const metadata: Metadata = {
  title: "AutoOps天枢",
  description: "AutoOps天枢智能运维平台",
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
