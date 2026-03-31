import { Container } from "@/models/Container";

export type DeployPayload = {
  composeProject?: string;
  projectPath: string;
  repoUrl: string;
  branch: string;
  composeFile: string;
  services: string[];
  gitUsername: string;
  gitToken: string;
  bootstrap: boolean;
};

export type DeployStatus = {
  runId: string;
  containerId: string;
  state: "pending" | "running" | "success" | "failed";
  message?: string;
  startedAt: string;
  finishedAt?: string;
  exitCode: number;
};

export type DeployLogChunk = {
  runId: string;
  offset: number;
  lines: string[];
  next: number;
  done: boolean;
};

const labels = {
  enabled: "dev.dozzle.deploy.enabled",
  path: "dev.dozzle.deploy.path",
  repo: "dev.dozzle.deploy.repo",
  branch: "dev.dozzle.deploy.branch",
  compose: "dev.dozzle.deploy.compose",
  service: "dev.dozzle.deploy.service",
} as const;

export const useDeploy = (container: Ref<Container>) => {
  const basePath = computed(() => `/api/hosts/${container.value.host}/containers/${container.value.id}/deploy`);

  const composeProject = computed(
    () => container.value.labels["com.docker.compose.project"] || container.value.labels["com.docker.stack.namespace"] || "",
  );

  const defaults = computed(() => ({
    enabled: container.value.labels[labels.enabled] === "true",
    projectPath: container.value.labels[labels.path] ?? "",
    repoUrl: container.value.labels[labels.repo] ?? "",
    branch: container.value.labels[labels.branch] ?? "main",
    composeFile: container.value.labels[labels.compose] ?? "docker-compose.yml",
    services: (container.value.labels[labels.service] ?? "")
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean),
    gitUsername: "x-access-token",
    bootstrap: true,
  }));

  async function saveCredentials(gitUsername: string, gitToken: string) {
    const response = await fetch(withBase(`${basePath.value}/credentials`), {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ gitUsername, gitToken }),
    });
    if (!response.ok) {
      throw new Error(await response.text());
    }
  }

  async function start(payload: DeployPayload) {
    if (payload.gitToken) {
      await saveCredentials(payload.gitUsername, payload.gitToken);
    }

    const response = await fetch(withBase(basePath.value), {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ ...payload, composeProject: payload.composeProject ?? composeProject.value }),
    });
    if (!response.ok) {
      throw new Error(await response.text());
    }
    return (await response.json()) as { runId: string };
  }

  async function status(runId: string) {
    const response = await fetch(withBase(`${basePath.value}/${runId}`));
    if (!response.ok) {
      throw new Error(await response.text());
    }
    return (await response.json()) as DeployStatus;
  }

  async function logs(runId: string, offset = 0) {
    const response = await fetch(withBase(`${basePath.value}/${runId}/logs?offset=${offset}`));
    if (!response.ok) {
      throw new Error(await response.text());
    }
    return (await response.json()) as DeployLogChunk;
  }

  async function history(limit = 20) {
    const response = await fetch(withBase(`${basePath.value}/history?limit=${limit}`));
    if (!response.ok) {
      throw new Error(await response.text());
    }
    const { items } = (await response.json()) as { items: DeployStatus[] };
    return items;
  }

  async function listComposeServices(projectPath: string, composeFile: string) {
    const response = await fetch(withBase(`${basePath.value}/services`), {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ composeProject: composeProject.value, projectPath, composeFile }),
    });
    if (!response.ok) return [];
    const { services } = (await response.json()) as { services: string[] };
    return services;
  }

  async function getConfig() {
    const query = composeProject.value ? `?composeProject=${encodeURIComponent(composeProject.value)}` : "";
    const response = await fetch(withBase(`${basePath.value}/config${query}`));
    if (!response.ok) {
      throw new Error(await response.text());
    }
    return (await response.json()) as {
      composeProject: string;
      config: {
        composeProject: string;
        projectPath: string;
        repoUrl: string;
        branch: string;
        composeFile: string;
        services?: string[];
      } | null;
    };
  }

  async function saveConfig(payload: {
    composeProject: string;
    projectPath: string;
    repoUrl: string;
    branch: string;
    composeFile: string;
    services: string[];
  }) {
    const response = await fetch(withBase(`${basePath.value}/config`), {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!response.ok) {
      throw new Error(await response.text());
    }
  }

  return { defaults, composeProject, start, status, logs, history, listComposeServices, getConfig, saveConfig };
};

