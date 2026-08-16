# LuxProvide / MeluXina startup-access brief

**Reviewed:** 2026-08-16

**Status:** research lead and PoC input; not an entitlement, allocation, contract,
or production-readiness decision.

**Canonical product contract:** [`PORTAL_PRD.md`](../docs/prd/PORTAL_PRD.md), especially
the MeluXina evidence boundary in `A5.1/A5.2`. This brief is intentionally
shorter and easier to scan; the PRD remains normative.

## Executive decision

Alzette should investigate LuxProvide’s **INITIATE** programme as the first
access hypothesis. The published offer is potentially sufficient for an
open-weight inference experiment, but it would provide project-based,
scheduled HPC allocation—not a guaranteed dedicated machine or always-on
customer endpoint.

The expected first experiment is one approved model in an Apptainer container,
served with a documented runtime such as vLLM or Triton, exercised with
synthetic/public data through SSH forwarding or another provider-approved path.
It must prove whether a persistent Alzette gateway-to-model path is possible
before any MeluXina hosting or customer-serving claim.

No application, account, reservation, contact, credential entry, or deployment
has been made.

## What the current startup programmes publish

| Route | Published fact | Alzette interpretation |
|---|---|---|
| INITIATE | Free access for startups refining ideas/prototypes; current page says CPU/GPU node-hours, storage, and up to six months. The current fact sheet specifies **4,000 CPU or 1,500 GPU node-hours, 10 TB storage, 24 L1/L2 support tickets, and up to six months**. | Best immediate PoC hypothesis, subject to acceptance and hardware availability. |
| INITIATE eligibility | Founded within the last five years; no previous credits beyond the free trial; no previous commercial agreement with LuxProvide; hardware availability applies. The current fact sheet says startups outside Luxembourg may also apply. | Alzette’s legal-entity age and prior LuxProvide history are still founder/operator facts to verify. Luxembourg incorporation alone does not prove eligibility. |
| Cashback80 | Up to 80% cashback for SMEs; a state-aid request under the Innovation aid for SMEs (RDI) scheme must be submitted before activation, with Luxinnovation assistance; general state-aid rules apply. | Possible follow-on economics, not a guaranteed discount, cash payment, or baseline margin assumption. |
| National/commercial access | Commercial access is open to industry, public administration, public research, and academia; special startup tracks exist. New projects require approval and onboarding. | A direct commercial route exists if INITIATE is unavailable, but public sources do not state the price, lead time, minimum allocation, or serving terms. |
| EuroHPC AI Factory Industrial Innovation | Playground access is continuously open, intended for entry-level/small AI allocations, and grants one-, two-, or three-month periods. MeluXina is included among covered systems. Commercial companies, SMEs, and startups can be eligible subject to the call rules and technical assessment. | Viable fallback/parallel route; free access and commercial exploitation are stated subject to eligibility and hosting-entity terms. Reporting, acknowledgement, and possible publication obligations apply. |

The current public programme page does not publish a rate card, exact cash value,
guaranteed allocation, approval SLA, or persistent-service commitment. An older
official PDF advertised “€25,000” and one year; the newer September 2025 fact
sheet says up to six months and does not state that amount. Do not quote the old
terms as current entitlement.

## What the infrastructure can demonstrate

Official documentation supports:

- Slurm-managed CPU, GPU, FPGA, and large-memory partitions;
- a documented GPU partition with 200 nodes and four NVIDIA A100-40 GPUs per
  node;
- full-node scheduled jobs, SSH access, project storage, scratch storage, and
  Apptainer container execution;
- vLLM and Triton LLM-serving examples, including querying an inference server
  through SSH port forwarding;
- a Cloud module and web-service interfaces, but not that INITIATE includes a
  persistent Cloud GPU VM, public ingress, load balancing, or an Alzette-ready
  always-on service.

This proves a credible **technical serving experiment**. It does not prove
queue latency, persistent uptime, external reachability, stable private
networking, customer isolation, production support, or customer-data
suitability.

MeluXina-AI is a future offering advertised for launch at the end of 2026. It
must not be used as current infrastructure evidence.

## Internal eligibility and PoC checklist

Before any external action, prepare internally:

1. legal entity name, VAT/PIC if relevant, incorporation date, and startup/SME
   status;
2. confirmation that Alzette has no prior LuxProvide commercial agreement or
   non-free credits;
3. one open-weight model, pinned version, licence record, and model-size/GPU
   fit;
4. a reproducible Apptainer image and vLLM/Triton launch record;
5. synthetic/public test data only;
6. an initial request for the smallest useful GPU allocation, without assuming
   a named or reserved machine;
7. measurements for queue wait, cold start, warm latency, throughput, errors,
   restart/recovery, allocation consumption, and actual/credited cost;
8. a decision output: **PoC-only**, **suitable for first pilot**, or
   **unsuitable**.

If MeluXina cannot expose a stable service directly, Alzette must own the
gateway, authentication, tenant isolation, metering, and customer API boundary
outside or alongside the provider layer. A Slurm job, SSH tunnel, Project ID,
VM ID, or provider credential is never a customer tenant or customer API.

## Questions that remain open

Ask LuxProvide only after an approved provider discussion exists:

- Does INITIATE permit a long-running open-weight inference server, or only
  scheduled/batch/interactive experiments?
- Which resource does INITIATE actually cover: standard A100 GPU nodes, Cloud
  GPU nodes, or another class? Can a project receive a persistent VM?
- Are public ingress, private connectivity, stable addresses, TLS, firewall
  allow-lists, and Alzette-to-model networking supported?
- What are queue, walltime, requeue/eviction, idle, restart, and maintenance
  rules for a serving process?
- What current terms cover commercial customer-serving inference, model/data
  ownership, confidentiality, retention, backups, logs, subprocessors, and
  incident/support obligations?
- What exactly is included in the allocation: hours, storage, support, expiry,
  unused-credit treatment, and overage pricing?
- What are the current Cashback80 eligible costs, approval/reimbursement timing,
  cap, clawback, and repeatability?
- Can LuxProvide provide machine-readable allocation, usage, queue, and cost
  records for Alzette reconciliation?

## Sources (official; accessed 2026-08-16)

- [LuxProvide startup and SME programmes](https://www.luxprovide.lu/programs/)
- [INITIATE/Cashback80 fact sheet, September 2025](https://www.luxprovide.lu/wp-content/uploads/2025/09/LuxProvide_Word_Website_Pages_Print_Template_Doc-1.pdf)
- [MeluXina access procedures](https://docs.lxp.lu/access/gaining_access/)
- [MeluXina allocations and monitoring](https://docs.lxp.lu/access/allocation_monitoring/)
- [MeluXina system overview and hardware](https://docs.lxp.lu/system/overview/)
- [MeluXina quick start and Slurm partitions](https://docs.lxp.lu/first-steps/quick_start/)
- [MeluXina platform usage policy](https://docs.lxp.lu/access/PoliciesSummary/)
- [MeluXina vLLM inference example](https://docs.lxp.lu/howto/llama3-vllm/)
- [MeluXina Triton inference example](https://docs.lxp.lu/howto/llama3-triton/)
- [EuroHPC AI Factory Industrial Innovation access terms](https://www.eurohpc-ju.europa.eu/document/download/bd7aa666-bdf3-4436-b5ec-0b34a781e817_en?filename=Terms+of+Reference-AIF+Access+Calls.pdf&prefLang=fi)
- [Luxembourg AI Factory acceleration programmes](https://aifactory.lu/Assess-Accelerate/Acceleration-programme)
- [LuxProvide MeluXina-AI future offering](https://www.luxprovide.lu/meluxina-ai/)
