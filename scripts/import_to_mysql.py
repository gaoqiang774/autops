#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
AutoOps - 一键导入 v360 数据至 MySQL 数据库工具
使用方法:
  python scripts/import_to_mysql.py --host 127.0.0.1 --port 3306 --user root --password yourpassword --database autops
或直接设置环境变量:
  set MYSQL_HOST=127.0.0.1
  set MYSQL_USER=root
  set MYSQL_PASSWORD=yourpass
  set MYSQL_DATABASE=autops
  python scripts/import_to_mysql.py
"""
import os
import sys
import argparse

def main():
    parser = argparse.ArgumentParser(description="一键导入 v360 数据到 MySQL")
    parser.add_argument("--host", default=os.getenv("MYSQL_HOST", "127.0.0.1"), help="MySQL 主机地址")
    parser.add_argument("--port", type=int, default=int(os.getenv("MYSQL_PORT", "3306")), help="MySQL 端口")
    parser.add_argument("--user", default=os.getenv("MYSQL_USER", "root"), help="MySQL 用户名")
    parser.add_argument("--password", default=os.getenv("MYSQL_PASSWORD", ""), help="MySQL 密码")
    parser.add_argument("--database", default=os.getenv("MYSQL_DATABASE", "autops"), help="MySQL 数据库名称")
    parser.add_argument("--sql-file", default="db/init_mysql_v360.sql", help="SQL 导入文件路径")
    args = parser.parse_args()

    sql_path = os.path.abspath(args.sql_file)
    if not os.path.exists(sql_path):
        print(f"错误: 找不到 SQL 文件: {sql_path}")
        sys.exit(1)

    print(f"正在连接 MySQL 数据库: {args.user}@{args.host}:{args.port}/{args.database} ...")
    try:
        import pymysql
    except ImportError:
        # Fallback to subprocess mysql command if pymysql not installed
        import subprocess
        print("未检测到 pymysql 模块，尝试调用 mysql CLI 执行导入...")
        cmd = ["mysql", f"-h{args.host}", f"-P{args.port}", f"-u{args.user}"]
        if args.password:
            cmd.append(f"-p{args.password}")
        cmd.append(args.database)
        try:
            with open(sql_path, "r", encoding="utf-8") as f:
                res = subprocess.run(cmd, stdin=f, capture_output=True, text=True)
            if res.returncode == 0:
                print("导入成功！已将全部 360 数据写入 MySQL 数据库。")
                return
            else:
                print(f"执行失败: {res.stderr}")
                sys.exit(res.returncode)
        except Exception as e:
            print(f"执行 mysql CLI 出错: {e}")
            print("提示: 可直接运行命令: mysql -h" + args.host + " -u" + args.user + " -p " + args.database + " < " + sql_path)
            sys.exit(1)

    # If pymysql is installed
    try:
        conn = pymysql.connect(
            host=args.host,
            port=args.port,
            user=args.user,
            password=args.password,
            database=args.database,
            charset='utf8mb4',
            client_flag=pymysql.constants.CLIENT.MULTI_STATEMENTS
        )
        with conn.cursor() as cursor:
            with open(sql_path, "r", encoding="utf-8") as f:
                sql = f.read()
            cursor.execute(sql)
            conn.commit()
        conn.close()
        print("导入成功！已将全部 360 数据写入 MySQL 数据库。")
    except Exception as e:
        print(f"MySQL 连接或执行错误: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
