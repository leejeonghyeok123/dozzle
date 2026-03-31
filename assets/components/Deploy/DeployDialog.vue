<template>
  <div class="space-y-4 p-4">
    <div>
      <h2 class="text-xl font-semibold">Deploy {{ container.name }}</h2>
      <p class="text-base-content/60 text-sm">Git pull + docker compose up -d --build</p>
    </div>

    <fieldset class="fieldset">
      <legend class="fieldset-legend">Project path</legend>
      <input v-model="form.projectPath" class="input input-bordered w-full" placeholder="/opt/apps/myapp" />
    </fieldset>

    <fieldset class="fieldset">
      <legend class="fieldset-legend">GitHub repo URL</legend>
      <input
        v-model="form.repoUrl"
        class="input input-bordered w-full"
        placeholder="https://github.com/org/repo.git"
      />
    </fieldset>

    <div class="grid grid-cols-2 gap-2">
      <fieldset class="fieldset">
        <legend class="fieldset-legend">Branch</legend>
        <input v-model="form.branch" class="input input-bordered w-full" />
      </fieldset>
      <fieldset class="fieldset">
        <legend class="fieldset-legend">Service(optional)</legend>
        <input v-model="form.service" class="input input-bordered w-full" />
      </fieldset>
    </div>

    <fieldset class="fieldset">
      <legend class="fieldset-legend">Compose file</legend>
      <input v-model="form.composeFile" class="input input-bordered w-full" />
    </fieldset>

    <div class="grid grid-cols-2 gap-2">
      <fieldset class="fieldset">
        <legend class="fieldset-legend">Git username</legend>
        <input v-model="form.gitUsername" class="input input-bordered w-full" />
      </fieldset>
      <fieldset class="fieldset">
        <legend class="fieldset-legend">Git token</legend>
        <input v-model="form.gitToken" type="password" class="input input-bordered w-full" />
      </fieldset>
    </div>

    <label class="label cursor-pointer justify-start gap-2">
      <input v-model="form.bootstrap" type="checkbox" class="checkbox checkbox-sm" />
      <span class="label-text">Bootstrap if folder/repo does not exist</span>
    </label>

    <div class="flex gap-2">
      <button class="btn btn-primary btn-sm" :disabled="running || !form.projectPath" @click="runDeploy">
        {{ running ? "Deploying..." : "Deploy" }}
      </button>
      <button class="btn btn-ghost btn-sm" :disabled="loadingHistory" @click="refreshHistory">Refresh History</button>
    </div>

    <div v-if="currentStatus" class="rounded border border-base-content/20 p-2 text-sm">
      <div><strong>Status:</strong> {{ currentStatus.state }}</div>
      <div v-if="currentStatus.message"><strong>Message:</strong> {{ currentStatus.message }}</div>
      <div><strong>Run ID:</strong> {{ currentStatus.runId }}</div>
    </div>

    <div v-if="recentLogs.length" class="rounded border border-base-content/20 p-2">
      <div class="mb-1 text-sm font-semibold">Recent logs</div>
      <pre class="bg-base-200 max-h-56 overflow-auto rounded p-2 text-xs">{{ recentLogs.join("\n") }}</pre>
    </div>

    <div class="rounded border border-base-content/20 p-2">
      <div class="mb-2 text-sm font-semibold">Recent deploy history</div>
      <div v-if="historyItems.length === 0" class="text-base-content/60 text-sm">No deploy runs yet.</div>
      <ul v-else class="space-y-1 text-sm">
        <li v-for="item in historyItems" :key="item.runId" class="flex items-center justify-between gap-2">
          <span class="truncate">{{ item.runId }}</span>
          <span class="badge badge-ghost">{{ item.state }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Container } from "@/models/Container";

const { container } = defineProps<{ container: Container }>();
const { showToast } = useToast();
const deploy = useDeploy(toRef(() => container));

const running = ref(false);
const loadingHistory = ref(false);
const currentRunId = ref("");
const currentStatus = ref<Awaited<ReturnType<typeof deploy.status>> | null>(null);
const recentLogs = ref<string[]>([]);
const historyItems = ref<Awaited<ReturnType<typeof deploy.history>>>([]);

const form = reactive({
  projectPath: deploy.defaults.value.projectPath,
  repoUrl: deploy.defaults.value.repoUrl,
  branch: deploy.defaults.value.branch,
  composeFile: deploy.defaults.value.composeFile,
  service: deploy.defaults.value.service,
  gitUsername: deploy.defaults.value.gitUsername,
  gitToken: "",
  bootstrap: deploy.defaults.value.bootstrap,
});

async function refreshHistory() {
  loadingHistory.value = true;
  try {
    historyItems.value = await deploy.history(20);
  } catch (error) {
    showToast({ type: "error", title: "Deploy", message: "Failed to load deploy history" });
  } finally {
    loadingHistory.value = false;
  }
}

async function pollStatus() {
  if (!currentRunId.value) return;
  const status = await deploy.status(currentRunId.value);
  currentStatus.value = status;
  const chunk = await deploy.logs(currentRunId.value, 0);
  recentLogs.value = chunk.lines.slice(-80);
  if (status.state === "success") {
    showToast({ type: "info", title: "Deploy", message: "Deployment succeeded" });
    await refreshHistory();
    return;
  }
  if (status.state === "failed") {
    showToast({ type: "error", title: "Deploy", message: status.message || "Deployment failed" });
    await refreshHistory();
    return;
  }
  window.setTimeout(pollStatus, 2000);
}

async function runDeploy() {
  running.value = true;
  recentLogs.value = [];
  try {
    const { runId } = await deploy.start({
      projectPath: form.projectPath,
      repoUrl: form.repoUrl,
      branch: form.branch,
      composeFile: form.composeFile,
      service: form.service,
      gitUsername: form.gitUsername,
      gitToken: form.gitToken,
      bootstrap: form.bootstrap,
    });
    currentRunId.value = runId;
    currentStatus.value = await deploy.status(runId);
    showToast({ type: "info", title: "Deploy", message: `Started (${runId})` });
    window.setTimeout(pollStatus, 1500);
  } catch (error) {
    const message = error instanceof Error ? error.message : "Failed to start deployment";
    showToast({ type: "error", title: "Deploy", message });
  } finally {
    running.value = false;
  }
}

onMounted(refreshHistory);
</script>

