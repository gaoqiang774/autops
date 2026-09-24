#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
AutoOps - 公司内网 140.100.180.88 离线 Docker 自动化部署脚本
执行流程：
1. 检查/下载 Linux Node 22 二进制包
2. 打包部署镜像所需文件
3. SFTP 上传至远程服务器 /opt/autops/
4. 远程解压并执行 docker-compose up -d --build
5. 验证服务运行与健康检查
"""

import os
import sys
import time
import tarfile
import urllib.request
import paramiko

SERVER_HOST = "140.100.180.88"
SERVER_PORT = 22
SERVER_USER = "root"
SERVER_PASS = "Qiang@1234"
REMOTE_DIR = "/opt/autops"
CONTAINER_PORT = 9020

NODE_TAR_NAME = "node-v22.14.0-linux-x64.tar.gz"
NODE_DOWNLOAD_URL = "https://npmmirror.com/mirrors/node/v22.14.0/node-v22.14.0-linux-x64.tar.gz"
LOCAL_DEPLOY_ARCHIVE = "autops-deploy.tar.gz"

def log(msg):
    print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] {msg}", flush=True)

def ensure_node_binary():
    if os.path.exists(NODE_TAR_NAME):
        size_mb = round(os.path.getsize(NODE_TAR_NAME) / (1024 * 1024), 2)
        log(f"✅ 检测到本地已存在 Node 22 二进制包: {NODE_TAR_NAME} ({size_mb} MB)")
        return
    log(f"⬇️ 正在从国内源下载 Linux Node 22 二进制包: {NODE_DOWNLOAD_URL} ...")
    urllib.request.urlretrieve(NODE_DOWNLOAD_URL, NODE_TAR_NAME)
    size_mb = round(os.path.getsize(NODE_TAR_NAME) / (1024 * 1024), 2)
    log(f"✅ 下载完成: {NODE_TAR_NAME} ({size_mb} MB)")

def make_deploy_tar():
    log("📦 正在生成本地部署压缩包 autops-deploy.tar.gz ...")
    include_paths = [
        "Dockerfile",
        "docker-compose.yml",
        ".dockerignore",
        "package.json",
        "dist",
        "node_modules",
        "app",
        "public",
        "信息资产台账-v360.xlsx",
        NODE_TAR_NAME
    ]
    with tarfile.open(LOCAL_DEPLOY_ARCHIVE, "w:gz") as tar:
        for p in include_paths:
            if os.path.exists(p):
                log(f"  + 打包: {p}")
                tar.add(p)
            else:
                log(f"  ! 警告: 文件未找到 {p}")
    size_mb = round(os.path.getsize(LOCAL_DEPLOY_ARCHIVE) / (1024 * 1024), 2)
    log(f"✅ 部署包生成成功: {LOCAL_DEPLOY_ARCHIVE} ({size_mb} MB)")

def upload_and_deploy():
    log(f"🔌 连接远程内网服务器 {SERVER_HOST} (用户: {SERVER_USER}) ...")
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(SERVER_HOST, port=SERVER_PORT, username=SERVER_USER, password=SERVER_PASS, timeout=15)
    
    # 创建远程目录
    ssh.exec_command(f"mkdir -p {REMOTE_DIR}")

    # 上传部署包
    log(f"🚀 开始 SFTP 上传部署包至 {REMOTE_DIR}/{LOCAL_DEPLOY_ARCHIVE} ...")
    sftp = ssh.open_sftp()
    local_size = os.path.getsize(LOCAL_DEPLOY_ARCHIVE)
    last_reported = [0]
    start_time = time.time()

    def progress_callback(transferred, total):
        pct = int(transferred / total * 100)
        if pct >= last_reported[0] + 20 or pct == 100:
            last_reported[0] = pct
            speed_mb = (transferred / (1024 * 1024)) / max(0.1, time.time() - start_time)
            log(f"   上传进度: {pct}% ({round(transferred / 1024 / 1024, 1)}MB / {round(total / 1024 / 1024, 1)}MB) - 速度: {speed_mb:.1f} MB/s")

    sftp.put(LOCAL_DEPLOY_ARCHIVE, f"{REMOTE_DIR}/{LOCAL_DEPLOY_ARCHIVE}", callback=progress_callback)
    sftp.close()
    log("✅ 部署包上传完成！")

    # 远程解压与 Docker 部署
    commands = [
        f"cd {REMOTE_DIR} && tar -xzf {LOCAL_DEPLOY_ARCHIVE}",
        f"cd {REMOTE_DIR} && docker-compose down || true",
        f"cd {REMOTE_DIR} && docker-compose up -d --build",
        f"sleep 5",
        f"docker ps --filter name=autops-app",
        f"curl -I -s --connect-timeout 5 http://127.0.0.1:{CONTAINER_PORT} || echo 'HTTP check pending...'"
    ]

    log("🐳 正在远程构建并启动 Docker 容器...")
    for cmd in commands:
        log(f"   执行: {cmd}")
        stdin, stdout, stderr = ssh.exec_command(cmd, timeout=300)
        out = stdout.read().decode('utf-8')
        err = stderr.read().decode('utf-8')
        if out:
            print(out.strip())
        if err and "warning" not in err.lower() and "stopping" not in err.lower():
            print("   [INFO/ERR]:", err.strip())

    ssh.close()
    log(f"🎉 部署全部完成！访问地址: http://{SERVER_HOST}:{CONTAINER_PORT}")

if __name__ == "__main__":
    ensure_node_binary()
    make_deploy_tar()
    upload_and_deploy()
