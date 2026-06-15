// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo, useCallback, useState, useRef } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  AlertTriangle,
  Bot,
  Download,
  MessageSquare,
  Plus,
  Rocket,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Sparkles,
} from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import { CopyButton } from "@/components/ui/copy-button";
import { StatusIndicator } from "@/components/ui/status-indicator";
import { useWebSocket } from "@/hooks/useWebSocket";
import { useAgentRunResult } from "@/hooks/useAgentRunResult";
import { useAgents } from "@/hooks/useAgents";
import { useAgentSession } from "@/hooks/useAgentSessions";
import { AgentModelBadge } from "@/components/agents/AgentModelBadge";
import { AgentMarkdown } from "@/components/agents/AgentMarkdown";
import {
  invokeAgent,
  getAgentRunResult,
  type AgentRun,
  type AgentRunResult,
  type PolicyValidation,
  type PolicyViolation,
} from "@/api/agent-runs";
import { downloadBlob } from "@/api/compliance";
import { cn } from "@/lib/utils";
import {
  extractYamlBlock,
  stripCodeBlocks,
  parseSpec,
  isRGDSpec,
  summarizeRGDSpec,
} from "./spec-extract";
import { SpecPreview } from "./SpecPreview";
import { useCanI } from "@/hooks/useCanI";

/** One structured question from kagent's adk_request_confirmation data part. */
interface KagentQuestion {
  question: string;
  choices: string[];
  multiple?: boolean;
}

/**
 * Extract kagent clarifying questions from the raw data parts of an
 * input-required run result. kagent emits a single `adk_request_confirmation`
 * data part whose questions live at args.originalFunctionCall.args.questions[].
 */
function extractKagentQuestions(dataParts: unknown[] | undefined): KagentQuestion[] {
  if (!dataParts?.length) return [];
  for (const part of dataParts) {
    if (typeof part !== "object" || part === null) continue;
    const p = part as Record<string, unknown>;
    if (p.name !== "adk_request_confirmation") continue;
    const args = p.args as Record<string, unknown> | undefined;
    const origCall = args?.originalFunctionCall as Record<string, unknown> | undefined;
    const innerArgs = origCall?.args as Record<string, unknown> | undefined;
    const qs = innerArgs?.questions;
    if (!Array.isArray(qs)) continue;
    return qs
      .filter((q) => typeof q === "object" && q !== null && "question" in q)
      .map((q) => {
        const qObj = q as Record<string, unknown>;
        return {
          question: String(qObj.question ?? ""),
          choices: Array.isArray(qObj.choices) ? (qObj.choices as string[]) : [],
          multiple: Boolean(qObj.multiple),
        };
      });
  }
  return [];
}

/** One conversation turn: a submitted prompt and its terminal outcome. */
export interface Turn {
  runId: string;
  prompt: string;
  result: AgentRunResult | null;
  timedOut: boolean;
}

/**
 * Kagent-style unified agent chat page (Story 50.x refactor).
 * Replaces RGDBuilderPage + SessionReplayPage. The sessionId URL param IS the
 * conversationId: the server groups all runs that share it into one session.
 * Navigating to a past session URL loads and displays its runs, then allows
 * new messages to be submitted into the same session context.
 */
export function AgentChatPage() {
  const { namespace, name, sessionId } = useParams<{ namespace: string; name: string; sessionId: string }>();
  const navigate = useNavigate();

  useWebSocket({ subscriptions: ["agent_runs"] });

  // Agent metadata comes from the live Casbin-scoped list (no static registry).
  // The URL params are the agent's namespace + CR name.
  const { data: agentsData } = useAgents();
  const agentMeta = agentsData?.agents.find((a) => a.namespace === namespace && a.name === name);
  const agentModel = agentMeta?.model;

  // Load the existing session runs for restoration
  const { data: session, isLoading: sessionLoading, isError: sessionError } = useAgentSession(sessionId);
  const lastRun = session?.runs?.at(-1) ?? null;

  // Fetch the last run's result to restore kagent context continuity
  const { data: lastRunResult } = useQuery({
    queryKey: ["agents", "runs", lastRun?.id, "result"],
    queryFn: () => getAgentRunResult(lastRun!.id),
    enabled: lastRun != null,
    refetchInterval: false,
    retry: false,
    staleTime: Infinity,
  });

  const [prompt, setPrompt] = useState("");
  // Turns submitted in THIS browser session (not loaded from server)
  const [newTurns, setNewTurns] = useState<Turn[]>([]);
  const [activeRun, setActiveRun] = useState<{ runId: string; prompt: string } | null>(null);
  const [invokeError, setInvokeError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const { result, timedOut } = useAgentRunResult(activeRun?.runId ?? null);
  const activeTerminal = activeRun != null && (result != null || timedOut);
  const inFlight = submitting || (activeRun != null && !activeTerminal);

  // Kagent context key for A2A continuity: prefer the active run's result,
  // then the last folded new turn, then the restored session's last run result.
  const contextKagentId =
    result?.kagentSessionId ??
    newTurns.at(-1)?.result?.kagentSessionId ??
    lastRunResult?.kagentSessionId ??
    "";

  // Extracted so both the main textarea submit and the questions form can invoke.
  const submitMessage = useCallback(async (message: string) => {
    const previousTurn: Turn | null =
      activeTerminal && activeRun
        ? { runId: activeRun.runId, prompt: activeRun.prompt, result: result ?? null, timedOut }
        : null;
    setInvokeError(null);
    setSubmitting(true);
    try {
      const run = await invokeAgent(namespace!, name!, message, sessionId!, contextKagentId || undefined);
      if (previousTurn) {
        setNewTurns((prev) => [...prev, previousTurn]);
      }
      setActiveRun({ runId: run.id, prompt: message });
      setPrompt("");
    } catch (error) {
      setInvokeError(error instanceof Error ? error.message : "Failed to invoke the agent");
    } finally {
      setSubmitting(false);
    }
  }, [activeRun, activeTerminal, result, timedOut, namespace, name, sessionId, contextKagentId]);

  const handleSubmit = useCallback(async () => {
    const message = prompt.trim();
    if (!message || inFlight) return;
    await submitMessage(message);
  }, [prompt, inFlight, submitMessage]);

  const handleNewConversation = useCallback(() => {
    navigate(
      `/agents/list/${encodeURIComponent(namespace!)}/${encodeURIComponent(name!)}/chat/${crypto.randomUUID()}`
    );
  }, [navigate, namespace, name]);

  const hasPastRuns = (session?.runs?.length ?? 0) > 0;
  const hasContent = hasPastRuns || newTurns.length > 0 || activeRun != null;

  return (
    <div className="space-y-6 max-w-4xl" data-testid="agent-chat-page">
      {/* Header */}
      <div className="space-y-2">
        <Link
          to="/agents/list"
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors md:hidden"
          data-testid="agent-chat-back"
        >
          ← Agents
        </Link>
        <div className="flex items-center gap-3 flex-wrap">
          <h1 className="text-2xl font-semibold text-foreground">
            {agentMeta?.name ?? name}
          </h1>
          <AgentModelBadge model={agentModel} />
          {hasContent && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleNewConversation}
              className="ml-auto"
              data-testid="agent-chat-new-conversation"
            >
              <Plus className="h-3.5 w-3.5 mr-1.5" aria-hidden="true" />
              New conversation
            </Button>
          )}
        </div>
        {agentMeta?.description && (
          <p className="text-sm text-muted-foreground">{agentMeta.description}</p>
        )}
      </div>

      {/* Past turns loaded from the session */}
      {sessionLoading && (
        <Skeleton className="h-32 w-full" data-testid="agent-chat-loading" />
      )}

      {sessionError && (
        <div
          className="rounded-lg border border-border py-6 text-center text-sm text-muted-foreground"
          data-testid="agent-chat-session-error"
        >
          This conversation could not be loaded.
        </div>
      )}

      {!sessionLoading && !sessionError && session && session.runs.length > 0 && (
        <div className="space-y-6" data-testid="agent-chat-past-turns">
          {session.runs.map((run, idx) => {
            // The last run is resumable when there's nothing new in-flight yet.
            // ReplayTurn only activates onSubmitAnswers when result.inputRequired.
            const resumable =
              idx === session.runs.length - 1 &&
              newTurns.length === 0 &&
              activeRun == null &&
              !inFlight;
            return (
              <ReplayTurn
                key={run.id}
                run={run}
                onSubmitAnswers={resumable ? submitMessage : undefined}
              />
            );
          })}
        </div>
      )}

      {/* New turns submitted in this browser session */}
      {newTurns.length > 0 && (
        <div className="space-y-6" data-testid="agent-chat-new-turns">
          {newTurns.map((turn) => (
            <TurnView key={turn.runId} turn={turn} agentName={name} />
          ))}
        </div>
      )}

      {/* Active run: live outcome once terminal, pulse while in-flight */}
      {activeRun &&
        (activeTerminal ? (
          <TurnView
            turn={{
              runId: activeRun.runId,
              prompt: activeRun.prompt,
              result: result ?? null,
              timedOut,
            }}
            onSubmitAnswers={result?.inputRequired && !inFlight ? submitMessage : undefined}
            agentName={name}
          />
        ) : (
          <div className="space-y-3" data-testid="agent-chat-inflight">
            <PromptBubble prompt={activeRun.prompt} />
            <AgentBubble agentName={name}>
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <StatusIndicator status="progressing" />
                Generating spec — inspecting cluster CRDs…
              </div>
            </AgentBubble>
          </div>
        ))}

      {/* Invoke failure */}
      {invokeError && (
        <ErrorAlert testId="agent-chat-invoke-error">
          <p className="font-medium">Could not start the agent</p>
          <p>{invokeError}</p>
          <p className="text-muted-foreground">
            Check that the kagent integration is healthy on the Agents page, then try again.
          </p>
        </ErrorAlert>
      )}

      {/* Prompt input */}
      <div className="space-y-2">
        <Textarea
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder='e.g. "I need a web app with Redis and a LoadBalancer"'
          rows={3}
          disabled={inFlight}
          data-testid="agent-chat-prompt"
          aria-label="Describe your infrastructure"
        />
        <div className="flex justify-end">
          <Button
            type="button"
            onClick={handleSubmit}
            disabled={inFlight || prompt.trim() === ""}
            data-testid="agent-chat-submit"
          >
            <Sparkles className="h-4 w-4 mr-1.5" aria-hidden="true" />
            {inFlight ? "Generating…" : "Generate spec"}
          </Button>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Shared presentational components
// ---------------------------------------------------------------------------

export function PromptBubble({ prompt }: { prompt: string }) {
  return (
    <div
      data-testid="agent-chat-user-prompt"
      className="rounded-lg border border-primary/30 bg-primary/5 px-4 py-3 text-sm whitespace-pre-wrap"
    >
      {prompt}
    </div>
  );
}

function AgentBubble({
  agentName,
  inputTokens,
  outputTokens,
  children,
}: {
  agentName?: string;
  inputTokens?: number;
  outputTokens?: number;
  children: React.ReactNode;
}) {
  const showTokens = inputTokens || outputTokens;
  return (
    <div className="rounded-lg border border-accent/30 bg-accent/5 overflow-hidden">
      {agentName && (
        <div className="flex items-center gap-1.5 px-4 py-2 border-b border-accent/20">
          <Bot className="h-3.5 w-3.5 text-accent" aria-hidden="true" />
          <span className="text-xs font-medium text-accent">kagent/{agentName}</span>
          {showTokens && (
            <span className="ml-auto text-xs text-muted-foreground tabular-nums">
              ↑{(inputTokens ?? 0).toLocaleString()} ↓{(outputTokens ?? 0).toLocaleString()} tokens
            </span>
          )}
        </div>
      )}
      <div className="px-4 py-3">
        {children}
      </div>
    </div>
  );
}

function ErrorAlert({ children, testId }: { children: React.ReactNode; testId: string }) {
  return (
    <div
      role="alert"
      data-testid={testId}
      className="flex gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm"
    >
      <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5 text-destructive" aria-hidden="true" />
      <div className="space-y-1">{children}</div>
    </div>
  );
}

/** Renders one terminal turn: spec preview, explanation, error or timeout. */
export const TurnView = memo(function TurnView({
  turn,
  onSubmitAnswers,
  agentName,
}: {
  turn: Turn;
  onSubmitAnswers?: (message: string) => void;
  agentName?: string;
}) {
  return (
    <div className="space-y-3">
      <PromptBubble prompt={turn.prompt} />
      <AgentBubble
        agentName={agentName}
        inputTokens={turn.result?.inputTokens}
        outputTokens={turn.result?.outputTokens}
      >
        <TurnOutcome turn={turn} onSubmitAnswers={onSubmitAnswers} />
      </AgentBubble>
    </div>
  );
});

function TurnOutcome({
  turn,
  onSubmitAnswers,
}: {
  turn: Turn;
  onSubmitAnswers?: (message: string) => void;
}) {
  if (turn.timedOut || turn.result == null) {
    return (
      <ErrorAlert testId="agent-chat-timeout-error">
        <p className="font-medium">The agent did not respond in time</p>
        <p>
          The agent may be under load or the kagent integration may be degraded. The run is
          recorded in Run History — try submitting again.
        </p>
      </ErrorAlert>
    );
  }

  if (turn.result.status === "failed") {
    return (
      <ErrorAlert testId="agent-chat-failed-error">
        <p className="font-medium">The agent run failed</p>
        {turn.result.error && <p className="font-mono text-xs">{turn.result.error}</p>}
        <p className="text-muted-foreground">
          Check that the kagent integration is healthy on the Agents page, then try again.
        </p>
      </ErrorAlert>
    );
  }

  if (turn.result.inputRequired) {
    const questions = extractKagentQuestions(turn.result.dataParts);
    return (
      <div data-testid="agent-chat-input-required" className="space-y-1">
        {turn.result.response && (
          <div>
            <AgentMarkdown>{turn.result.response}</AgentMarkdown>
          </div>
        )}
        {onSubmitAnswers ? (
          <QuestionsForm questions={questions} onSubmit={onSubmitAnswers} />
        ) : (
          <div className="space-y-3 border-t border-accent/20 pt-3 mt-1">
            <p className="text-sm font-medium text-foreground flex items-center gap-1.5">
              <MessageSquare className="h-4 w-4 shrink-0 text-accent" aria-hidden="true" />
              Questions for you
            </p>
            {questions.length > 0 ? (
              questions.map((q, i) => (
                <div key={i} className="space-y-1">
                  <p className="text-sm font-semibold text-foreground">{q.question}</p>
                  {q.choices.length > 0 && (
                    <div className="flex flex-wrap gap-1.5">
                      {q.choices.map((c) => (
                        <span key={c} className="px-3 py-0.5 rounded-full text-sm bg-muted/40 border border-border/60 text-muted-foreground">
                          {c}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              ))
            ) : (
              <p className="text-sm text-muted-foreground italic">No questions captured.</p>
            )}
          </div>
        )}
      </div>
    );
  }

  const yamlBlock = extractYamlBlock(turn.result.response);
  const explanation = stripCodeBlocks(turn.result.response);

  if (!yamlBlock) {
    return (
      <div data-testid="agent-chat-explanation">
        <AgentMarkdown>{explanation}</AgentMarkdown>
      </div>
    );
  }

  const parsedSpec = parseSpec(yamlBlock);
  const deployable = parsedSpec != null && isRGDSpec(parsedSpec);
  const pv = turn.result.policyValidation ?? null;

  return (
    <div className="space-y-3" data-testid="agent-chat-result">
      {explanation && (
        <AgentMarkdown className="text-muted-foreground">{explanation}</AgentMarkdown>
      )}
      <PolicySection pv={pv} deployable={deployable} />
      <SpecPreview yamlBlock={yamlBlock} />
      <SpecActions
        yamlBlock={yamlBlock}
        deployable={deployable}
        prompt={turn.prompt}
        runId={turn.runId}
        testIdPrefix="agent-chat"
      />
      {pv?.status === "failed" && pv.revisedResponse && (
        <RevisedSpecSection pv={pv} prompt={turn.prompt} runId={turn.runId} />
      )}
    </div>
  );
}

/**
 * Interactive kagent questions form. Each question has predefined chips
 * (clicking fills the per-question input) plus a free-text input. A single
 * "Continue" button formats all answers into one message and submits.
 */
function QuestionsForm({
  questions,
  onSubmit,
}: {
  questions: KagentQuestion[];
  onSubmit: (message: string) => void;
}) {
  const [answers, setAnswers] = useState<string[]>(() =>
    new Array(questions.length).fill("")
  );
  const inputRefs = useRef<(HTMLInputElement | null)[]>([]);

  const setAnswer = useCallback((i: number, value: string) => {
    setAnswers((prev) => {
      const next = [...prev];
      next[i] = value;
      return next;
    });
  }, []);

  const handleChip = useCallback((qi: number, choice: string, multiple: boolean) => {
    setAnswers((prev) => {
      const next = [...prev];
      if (multiple) {
        const parts = next[qi].trim() ? next[qi].split(", ") : [];
        const idx = parts.indexOf(choice);
        next[qi] = idx >= 0 ? parts.filter((_, j) => j !== idx).join(", ") : [...parts, choice].join(", ");
      } else {
        next[qi] = choice;
      }
      return next;
    });
    inputRefs.current[qi]?.focus();
  }, []);

  const handleContinue = useCallback(() => {
    const parts = questions
      .map((q, i) => answers[i].trim() ? `${q.question}: ${answers[i].trim()}` : null)
      .filter((s): s is string => s !== null);
    if (parts.length > 0) onSubmit(parts.join("\n"));
  }, [questions, answers, onSubmit]);

  const hasAnyAnswer = answers.some((a) => a.trim() !== "");

  return (
    <div
      className="rounded-lg border border-primary/30 bg-primary/5 px-4 py-4 space-y-4"
      data-testid="agent-chat-questions-form"
    >
      <p className="text-sm font-medium text-foreground flex items-center gap-1.5">
        <MessageSquare className="h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
        Questions for you
      </p>

      {questions.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          The agent needs more information. Type your answer below.
        </p>
      ) : (
        questions.map((q, i) => {
          const selectedParts = answers[i] ? answers[i].split(", ") : [];
          return (
            <div key={i} className="space-y-2">
              <p className="text-sm font-semibold text-foreground">{q.question}</p>
              {q.choices.length > 0 && (
                <div className="flex flex-wrap gap-1.5" role="group" aria-label={q.question}>
                  {q.choices.map((choice) => {
                    const active = selectedParts.includes(choice);
                    return (
                      <button
                        key={choice}
                        type="button"
                        onClick={() => handleChip(i, choice, q.multiple ?? false)}
                        className={cn(
                          "px-3 py-0.5 rounded-full text-sm border transition-colors",
                          active
                            ? "bg-primary/20 border-primary/40 text-foreground"
                            : "bg-muted/40 border-border/60 text-muted-foreground hover:bg-muted hover:text-foreground hover:border-border"
                        )}
                        aria-pressed={active}
                      >
                        {choice}
                      </button>
                    );
                  })}
                </div>
              )}
              <Input
                ref={(el) => { inputRefs.current[i] = el; }}
                value={answers[i]}
                onChange={(e) => setAnswer(i, e.target.value)}
                placeholder="Type your own answer"
                className="bg-muted/20"
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    if (i < questions.length - 1) {
                      inputRefs.current[i + 1]?.focus();
                    } else {
                      handleContinue();
                    }
                  }
                }}
                data-testid={`agent-chat-question-input-${i}`}
              />
            </div>
          );
        })
      )}

      <Button
        type="button"
        size="sm"
        onClick={handleContinue}
        disabled={!hasAnyAnswer}
        data-testid="agent-chat-questions-submit"
      >
        <Sparkles className="h-3.5 w-3.5 mr-1.5" aria-hidden="true" />
        Continue
      </Button>
    </div>
  );
}

function SpecActions({
  yamlBlock,
  deployable,
  prompt,
  runId,
  testIdPrefix,
}: {
  yamlBlock: string;
  deployable: boolean;
  prompt: string;
  runId: string;
  testIdPrefix: string;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const { allowed: canCreateRGD } = useCanI("rgds", "create");
  return (
    <div className="flex gap-2">
      {deployable && canCreateRGD && (
        <Button
          type="button"
          size="sm"
          onClick={() =>
            navigate("/deploy-rgd", {
              state: { specYaml: yamlBlock, requirement: prompt, runId, backgroundLocation: location },
            })
          }
          data-testid={`${testIdPrefix}-use-spec`}
        >
          <Rocket className="h-3.5 w-3.5 mr-1.5" aria-hidden="true" />
          Use this spec
        </Button>
      )}
      <CopyButton text={yamlBlock} label="Copy spec" data-testid={`${testIdPrefix}-copy`} />
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => downloadSpec(yamlBlock)}
        data-testid={`${testIdPrefix}-download`}
      >
        <Download className="h-3.5 w-3.5 mr-1.5" aria-hidden="true" />
        Download YAML
      </Button>
    </div>
  );
}

function SystemGatekeeperHeader() {
  return (
    <div className="flex items-center gap-1.5">
      <Shield className="h-3.5 w-3.5 text-amber-500" aria-hidden="true" />
      <span className="text-xs font-semibold text-amber-500 uppercase tracking-wide">
        System · Gatekeeper
      </span>
    </div>
  );
}

function PolicySection({ pv, deployable }: { pv: PolicyValidation | null; deployable: boolean }) {
  if (pv == null) {
    if (!deployable) return null;
    return (
      <div
        data-testid="agent-chat-policy-ee-notice"
        className="rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2 space-y-1"
      >
        <SystemGatekeeperHeader />
        <p className="text-xs text-muted-foreground">
          Policy validation requires an Enterprise license
        </p>
      </div>
    );
  }
  if (pv.status === "passed") {
    return <PolicyPassedBadge testId="agent-chat-policy-badge" />;
  }
  if (pv.status === "unavailable") {
    return (
      <div
        role="status"
        data-testid="agent-chat-policy-unavailable"
        className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2 space-y-1.5"
      >
        <SystemGatekeeperHeader />
        <div className="flex gap-2 items-start text-xs">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0 mt-0.5 text-amber-500" aria-hidden="true" />
          <span>Policy validation unavailable — {pv.reason || "Gatekeeper unreachable"}</span>
        </div>
      </div>
    );
  }
  return (
    <PolicyViolationsPanel
      violations={pv.violations ?? []}
      testId="agent-chat-policy-violations"
    />
  );
}

function PolicyPassedBadge({ testId }: { testId: string }) {
  return (
    <div
      data-testid={testId}
      className="rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2 space-y-1.5"
    >
      <SystemGatekeeperHeader />
      <p className="inline-flex items-center gap-1.5 text-xs font-semibold text-green-600 dark:text-green-400">
        <ShieldCheck className="h-3.5 w-3.5" aria-hidden="true" />
        Policy validated ✓
      </p>
    </div>
  );
}

function PolicyViolationsPanel({
  violations,
  testId,
}: {
  violations: PolicyViolation[];
  testId: string;
}) {
  return (
    <div
      role="alert"
      data-testid={testId}
      className="space-y-2 rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm"
    >
      <SystemGatekeeperHeader />
      <p className="flex items-center gap-1.5 font-medium text-destructive">
        <ShieldAlert className="h-4 w-4" aria-hidden="true" />
        Blocked by Gatekeeper policy
      </p>
      <ul className="space-y-1 text-xs">
        {violations.map((v, i) => (
          <li key={`${v.constraint}-${i}`}>
            <span className="font-mono">&quot;{v.constraint}&quot;</span>{" "}
            ({v.enforcementAction || "deny"}): {v.message}
            {v.resourceId && (
              <span className="ml-1.5 inline-flex items-center px-1.5 py-0.5 rounded bg-muted font-mono text-[10px]">
                {v.resourceId}
              </span>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}

function RevisedSpecSection({
  pv,
  prompt,
  runId,
}: {
  pv: PolicyValidation;
  prompt: string;
  runId: string;
}) {
  const revisedYaml = extractYamlBlock(pv.revisedResponse ?? "");
  const revisedExplanation = stripCodeBlocks(pv.revisedResponse ?? "");
  const revisedSpec = revisedYaml ? parseSpec(revisedYaml) : null;
  const revisedDeployable = revisedSpec != null && isRGDSpec(revisedSpec);

  return (
    <div className="space-y-3 border-t border-border/60 pt-3" data-testid="agent-chat-revised-spec">
      <p className="text-sm font-medium">Revised spec — automatic revision attempt</p>
      {pv.revisedStatus === "passed" && <PolicyPassedBadge testId="agent-chat-revised-badge" />}
      {pv.revisedStatus === "failed" && (
        <PolicyViolationsPanel
          violations={pv.revisedViolations ?? []}
          testId="agent-chat-revised-violations"
        />
      )}
      {revisedYaml ? (
        <>
          <SpecPreview yamlBlock={revisedYaml} />
          <SpecActions
            yamlBlock={revisedYaml}
            deployable={revisedDeployable}
            prompt={prompt}
            runId={runId}
            testIdPrefix="agent-chat-revised"
          />
        </>
      ) : (
        <div>
          <AgentMarkdown>{revisedExplanation}</AgentMarkdown>
        </div>
      )}
    </div>
  );
}

/**
 * One replayed turn from a past session: fetches the full result once
 * (non-polling) and renders via TurnView, with graceful degradation to the
 * run's recommendationSummary if the result has expired.
 */
function ReplayTurn({
  run,
  onSubmitAnswers,
}: {
  run: AgentRun;
  onSubmitAnswers?: (message: string) => void;
}) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["agents", "runs", run.id, "result"],
    queryFn: () => getAgentRunResult(run.id),
    refetchInterval: false,
    retry: false,
    staleTime: Infinity,
  });

  if (isLoading) {
    return (
      <div className="space-y-3" data-testid="agent-chat-replay-turn">
        <PromptBubble prompt={run.inputSummary} />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }

  if (data != null) {
    return (
      <div data-testid="agent-chat-replay-turn">
        <TurnView
          turn={{ runId: run.id, prompt: run.inputSummary, result: data, timedOut: false }}
          onSubmitAnswers={data.inputRequired ? onSubmitAnswers : undefined}
        />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-3" data-testid="agent-chat-replay-turn">
        <PromptBubble prompt={run.inputSummary} />
        <AgentBubble>
          <p className="text-sm text-muted-foreground" data-testid="agent-chat-replay-fetch-error">
            Could not load this response — this is usually transient.
          </p>
        </AgentBubble>
      </div>
    );
  }

  // data === null: result expired — degrade to recommendationSummary
  return (
    <div className="space-y-3" data-testid="agent-chat-replay-turn">
      <PromptBubble prompt={run.inputSummary} />
      <AgentBubble>
        <div data-testid="agent-chat-replay-summary-fallback">
          {run.recommendationSummary ? (
            <AgentMarkdown>{run.recommendationSummary}</AgentMarkdown>
          ) : (
            <p className="text-sm text-muted-foreground italic">No response recorded.</p>
          )}
          <p className="mt-2 text-xs text-muted-foreground" data-testid="agent-chat-replay-expired-note">
            Full response no longer available — showing the saved summary.
          </p>
        </div>
      </AgentBubble>
    </div>
  );
}

function downloadSpec(yamlBlock: string) {
  const spec = parseSpec(yamlBlock);
  const name = spec && isRGDSpec(spec) ? summarizeRGDSpec(spec).name : "";
  const safeName = name.replace(/[^a-zA-Z0-9._-]/g, "-").replace(/^[.-]+/, "");
  const filename = `${safeName || "rgd-spec"}.yaml`;
  downloadBlob(new Blob([yamlBlock], { type: "application/yaml" }), filename);
}
