Autor: Prof. Pedro Filho

# Kind Cluster + GitHub Actions Runner

Guia para criar um cluster Kind local com 1 control-plane e instalar o GitHub Actions Runner Controller (ARC) via Helm.

## Pré-requisitos

- [Docker](https://docs.docker.com/get-docker/)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/) >= 3.x

---

## 1. Criar o cluster Kind

```bash
kind create cluster --config kind-config.yaml
```

Verifique se o cluster está funcionando:

```bash
kubectl cluster-info --context kind-cicd-cluster
kubectl get nodes
```

---

## 2. Instalar o Actions Runner Controller (ARC)

O ARC é o controlador oficial da GitHub para runners auto-hospedados no Kubernetes.

> **Nota:** O projeto original foi descontinuado. O ARC v2 é distribuído via OCI pelo
> repositório `ghcr.io/actions/actions-runner-controller-charts`.

### Arquitetura: dois charts, papéis distintos

A instalação é composta por dois charts Helm com responsabilidades separadas:

| Chart | Papel | Instalações por cluster |
|---|---|---|
| `gha-runner-scale-set-controller` | Operador Kubernetes — observa a fila de jobs no GitHub e decide quantos runners criar ou destruir | **Uma única vez** |
| `gha-runner-scale-set` | Pool de runners — pods que executam os jobs e se registram em um repositório ou organização específica | **Uma por repo/org** |

**Analogia:** o controller é o gerente de RH; cada `runner-scale-set` é um time contratado para um projeto específico. O gerente existe uma vez, os times podem ser vários.

```
cluster
├── arc-systems/
│   └── arc  (controller)            ← gha-runner-scale-set-controller
└── arc-runners/
    ├── runner-set-repo-a            ← gha-runner-scale-set (aponta p/ repo A)
    └── runner-set-org-b             ← gha-runner-scale-set (aponta p/ org B)
```

### 2.1 Instalar o controller

```bash
helm install arc \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set-controller \
  --namespace arc-systems \
  --create-namespace
```

Confirme que o controller está rodando:

```bash
kubectl get pods -n arc-systems
```

---

## 3. Criar um Personal Access Token (PAT) no GitHub

Antes de instalar o runner você precisa de um token com as permissões corretas.

1. Acesse **GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)**
2. Clique em **Generate new token (classic)**
3. Selecione o escopo:
   - Para **repositório único**: `repo`
   - Para **organização**: `admin:org`
4. Copie o token gerado — ele só aparece uma vez

---

## 4. Conceder acesso à API do Kubernetes ao runner

Como o runner roda **dentro do próprio cluster**, ele pode autenticar na API do Kubernetes via **in-cluster config** — o `kubectl` detecta automaticamente que está dentro de um pod e usa o token do ServiceAccount montado em `/var/run/secrets/kubernetes.io/serviceaccount/`.

Isso elimina a necessidade de armazenar um `KUBE_CONFIG` como secret no GitHub.

### Como funciona

```
pod do runner
└── /var/run/secrets/kubernetes.io/serviceaccount/
    ├── token      ← token JWT do ServiceAccount
    ├── ca.crt     ← certificado do cluster
    └── namespace  ← namespace atual

kubectl lê esses arquivos + as env vars KUBERNETES_SERVICE_HOST e
KUBERNETES_SERVICE_PORT e se conecta à API sem nenhuma configuração extra.
```

### 4.1 Aplicar o manifesto RBAC

O arquivo [runner-rbac.yaml](runner-rbac.yaml) cria:

| Recurso | Nome | O que faz |
|---|---|---|
| `ServiceAccount` | `runner-sa` | identidade do pod do runner no cluster |
| `ClusterRoleBinding` | `runner-admin-binding` | vincula o `runner-sa` ao `cluster-admin`, concedendo acesso total ao cluster |

O `cluster-admin` é um `ClusterRole` built-in do Kubernetes que concede permissão `*` sobre todos os recursos, em todos os namespaces e em todos os apiGroups.

```bash
kubectl create ns arc-runners
kubectl apply -f runner-rbac.yaml
```

O `values-runner.yaml` já está configurado com `serviceAccountName: runner-sa` para que os pods do runner usem esse ServiceAccount.

> **Atenção:** `cluster-admin` é adequado para ambientes locais de desenvolvimento e estudo.
> Em produção, recomenda-se criar um `ClusterRole` customizado com apenas as permissões necessárias.

---

## 5. Configurar e instalar o Runner

Há duas formas de passar parâmetros na instalação — use uma delas ou combine as duas:

**Opção A — via arquivo de values (recomendado para múltiplos parâmetros):**

Voçê deve obter o arquivo **values** do repositório, editá-lo e utilizá-lo no comando de instalação.

Para ver todos os valores disponíveis no chart antes de customizá-los, use:

```bash
helm show values \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set
```

Para salvar como arquivo e editar:

```bash
helm show values \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set \
  > kind/values-runner.yaml
```

O repositório já inclui um [values-runner.yaml](values-runner.yaml) com os campos essenciais pré-selecionados. Abra-o e preencha:

```yaml
githubConfigUrl: "https://github.com/SEU_ORG/SEU_REPO"
# ou para organização:
# githubConfigUrl: "https://github.com/SEU_ORG"

githubConfigSecret:
  github_token: "ghp_SEU_TOKEN_AQUI"
```


```bash
helm install arc-runner-set \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set \
  --namespace arc-runners \
  --create-namespace \
  --values values-runner.yaml
```

**Opção B — via `--set` em linha de comando (útil para ajustes pontuais ou automação):**

```bash
helm install arc-runner-set \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set \
  --namespace arc-runners \
  --create-namespace \
  --set githubConfigUrl="https://github.com/SEU_ORG/SEU_REPO" \
  --set githubConfigSecret.github_token="ghp_SEU_TOKEN_AQUI" \
  --set minRunners=1 \
  --set maxRunners=5
```

As duas opções podem ser combinadas — `--set` sobrescreve o que estiver no arquivo:

```bash
helm install arc-runner-set \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set \
  --namespace arc-runners \
  --create-namespace \
  --values kind/values-runner.yaml \
  --set maxRunners=10
```

Verifique os pods do runner:

```bash
kubectl get pods -n arc-runners
```

---

## 6. Configurar o runner no repositório GitHub

Após a instalação, o runner aparecerá automaticamente no GitHub desde que o token e a URL estejam corretos.

Para confirmar:

1. Acesse o repositório no GitHub
2. Vá em **Settings → Actions → Runners**
3. O runner `arc-runner-set` deve aparecer com status **Idle**

---

## 7. Usar o runner em um workflow

No arquivo de workflow (`.github/workflows/*.yaml`), referencie o runner pelo nome definido no Helm release. Para este primeiro exemplo, vamos usar o conteúdo do arquivo [main.yaml](main.yaml)

```yaml
# Nome do workflow
name: primeira-pipeline

on: # Definindo a trigger
  push:
    branches: ["main"]
  workflow_dispatch: # Permite que seja executado manualmente

jobs:
  build:
    name: "Meu primeiro Job"
    runs-on: arc-runner-set  # nome do Helm release
    steps:
      - uses: actions/checkout@v4 # Action para ter acesso ao repositorio
      - name: "Primeira execução"
        run: echo "Primeira execução no kind"
      - name: "Segunda execução"
        run: |
          echo "Segunda execução no kind"
          echo "Parabéns"
```

---

## 8. Ajustar o número de runners (escalonamento)

Edite `values-runner.yaml` e altere os limites:

```yaml
minRunners: 1   # mínimo de runners em standby
maxRunners: 5   # máximo durante picos de carga
```

Aplique a atualização:

```bash
helm upgrade arc-runner-set \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set \
  --namespace arc-runners \
  --values values-runner.yaml
```

---

## 9. Destruir o cluster

```bash
kind delete cluster --name cicd-cluster
```

---

## Referências

- [Documentação oficial do ARC (v2)](https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners-with-actions-runner-controller/quickstart-for-actions-runner-controller)
- [Repositório do ARC no GitHub](https://github.com/actions/actions-runner-controller)
- [Documentação do Kind](https://kind.sigs.k8s.io/)
