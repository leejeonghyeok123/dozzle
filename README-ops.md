# Dozzle 운영 배포 정리 (Remote Agent)

이 문서는 운영 환경에서 Dozzle를 중앙 1대 + 원격 서버 N대로 구성하는 최소 절차를 정리합니다.

## 어떤 compose 파일을 어디서 쓰나

- 중앙 서버: `docker-compose.central.yml`만 실행
- 원격 서버(각 서버): `docker-compose.agent.yml`만 실행
- `docker-compose.yml`은 개발/테스트용 서비스가 섞여 있으므로 운영에서는 사용하지 않음

## 필요 서비스 (운영 최소 구성)

- 중앙: `dozzle` 1개
- 원격: `dozzle-agent` 1개씩

다음 서비스들은 운영 최소 구성에는 불필요:

- `custom_base`
- `simple-auth`
- `remote`
- `dozzle-with-agent`
- `playwright`

## 사전 준비

1. 중앙/원격 모두 동일한 인증서 파일 준비
   - `./certs/dozzle_cert.pem`
   - `./certs/dozzle_key.pem`
2. 네트워크/방화벽
   - 중앙 -> 원격 각 서버 `7007/tcp` 허용
3. 중앙 compose의 `DOZZLE_REMOTE_AGENT`를 실제 agent 주소로 수정
   - 예: `10.0.1.11:7007,10.0.1.12:7007`

### 인증서 생성 예시 (Git Bash/WSL)

```bash
mkdir -p certs
openssl genpkey -algorithm Ed25519 -out certs/dozzle_key.pem
openssl req -new -key certs/dozzle_key.pem -out certs/dozzle_request.csr -subj "/C=US/ST=California/L=San Francisco/O=Dozzle"
openssl x509 -req -in certs/dozzle_request.csr -signkey certs/dozzle_key.pem -out certs/dozzle_cert.pem -days 1825
rm certs/dozzle_request.csr
```

보안 주의:

- `certs/dozzle_key.pem`은 절대 Git에 커밋하지 마세요.
- 생성한 `dozzle_cert.pem`/`dozzle_key.pem`은 중앙 서버와 모든 agent 서버에 동일하게 배포해야 합니다.

## 실행 순서

1. 원격 서버들에서 agent 기동
   - `docker compose -f docker-compose.agent.yml up -d --build`
2. 중앙 서버에서 dozzle 기동
   - `docker compose -f docker-compose.central.yml up -d --build`

## 확인 방법

1. 중앙 Dozzle UI 접속 (`http://<central-ip>:8090`)
2. 호스트 목록에 원격 서버들이 표시되는지 확인
3. 컨테이너 로그 스트리밍 확인
4. `DOZZLE_ENABLE_ACTIONS=true` 설정 시 원격 컨테이너 start/stop/restart 동작 확인

## Deploy 버튼 사용 (신규)

Deploy 버튼은 컨테이너 메뉴에서 실행됩니다.

- 중앙 서버에 `DOZZLE_ENABLE_ACTIONS=true` 필요
- 최초 실행 시 UI에서 아래 값을 입력
  - `projectPath` (agent 컨테이너 내부 Linux 경로)
  - `repoUrl` (HTTPS)
  - `branch`
  - `composeFile`
  - GitHub username/token
- 토큰은 중앙 서버 `./data/deploy_credentials.enc`에 암호화 저장됨
  - 암호화 키: `DOZZLE_DEPLOY_SECRET_KEY` 환경변수

추가 필수 조건:

- agent 이미지에 `git`과 `docker compose` CLI가 포함되어 있어야 합니다.
- `projectPath`가 실제로 agent 컨테이너에서 접근 가능해야 합니다(볼륨 마운트 필요).
- Windows 경로(`D:\\...`) 대신 Linux 경로(예: `/workspace/ragregservice`)를 사용하세요.

컨테이너 라벨로 기본값 지정 가능:

- `dev.dozzle.deploy.enabled=true`
- `dev.dozzle.deploy.path=/opt/apps/myapp`
- `dev.dozzle.deploy.repo=https://github.com/org/repo.git`
- `dev.dozzle.deploy.branch=main`
- `dev.dozzle.deploy.compose=docker-compose.yml`
- `dev.dozzle.deploy.service=web`

## 자주 하는 실수

- 중앙에서 `docker-compose.yml` 실행함 (운영 기준 아님)
- 중앙과 원격의 인증서가 서로 다름
- 원격 서버 `7007` 포트 미개방
- `DOZZLE_REMOTE_AGENT`에 잘못된 IP/포트 입력

