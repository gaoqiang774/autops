# AutoOps - Dockerfile for Internal Environment
# Based on existing ubuntu:22.04 on internal host
FROM ubuntu:22.04

ENV TZ=Asia/Shanghai \
    DEBIAN_FRONTEND=noninteractive \
    NODE_ENV=production \
    PORT=3000

WORKDIR /app

# 安装本地解压的 Node.js 22
ADD node-v22.14.0-linux-x64.tar.gz /usr/local/
RUN mv /usr/local/node-v22.14.0-linux-x64 /usr/local/node && \
    ln -sf /usr/local/node/bin/node /usr/local/bin/node && \
    ln -sf /usr/local/node/bin/npm /usr/local/bin/npm && \
    ln -sf /usr/local/node/bin/npx /usr/local/bin/npx

# 复制生产代码、配置与产物
COPY package.json ./
COPY server.mjs ./
COPY dist ./dist
COPY node_modules ./node_modules
COPY public ./public
COPY app ./app
COPY 信息资产台账-v360.xlsx ./

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["node", "-e", "fetch('http://127.0.0.1:3000/').then(r => process.exit(r.ok ? 0 : 1)).catch(() => process.exit(1))"]

CMD ["node", "server.mjs"]
