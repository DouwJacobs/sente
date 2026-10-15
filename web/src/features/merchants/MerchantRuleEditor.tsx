import { useEffect, useRef, useState } from "react";
import { api } from "../../api";
import { MerchantLogoField } from "../../MerchantAvatar";
import { Button, Field, Form, Modal } from "../../ui";
import { CategoryChoice, ChoiceField } from "../../Choices";
import { PagedSelect } from "../../PagedList";
import { useTask } from "../../shared/useTask";
import type { PageProps, Row } from "../../shared/types";

export function MerchantRuleEditor({ rule, data, notify, refresh, onClose, onDone }: {
  rule?: Row; data: PageProps["data"]; notify: PageProps["notify"];
  refresh: () => void; onClose: () => void; onDone?: () => void;
}) {
  const { busy, run } = useTask(notify);
  const [edit, setEdit] = useState<Row>(() => rule
    ? { ...rule, account_id: rule.account_id ?? 0 }
    : { account_id: data.user.budget_member ? 0 : data.accounts.find(a => a.role === "editor")?.id || "",
        merchant_id: null, pattern: "", direction: "any", priority: 0, enabled: true });
  const [merchantName, setMerchantName] = useState("");
  const [nameError, setNameError] = useState("");
  const [categoryError, setCategoryError] = useState("");
  const [groupError, setGroupError] = useState("");
  const [logo, setLogo] = useState(rule?.merchant_logo || "");
  const [logoVersion, setLogoVersion] = useState(rule?.merchant_version || 1);
  const [logoDirty, setLogoDirty] = useState(false);
  const [logoBusy, setLogoBusy] = useState(false);
  const [merchantLoading, setMerchantLoading] = useState(false);
  const [category, setCategory] = useState<number | null>(rule?.category_id ?? null);
  const [group, setGroup] = useState<number | null>(rule?.spending_group_id ?? null);
  const request = useRef(0);
  useEffect(() => () => { request.current++; }, []);
  const update = (key: string, value: unknown) => setEdit(old => ({ ...old, [key]: value }));
  const clearMerchant = () => {
    request.current++;
    setMerchantLoading(false); setLogo(""); setLogoDirty(false);
    setCategory(null); setGroup(null); setCategoryError(""); setGroupError("");
  };
  const selectMerchant = async (id: number | null) => {
    clearMerchant(); update("merchant_id", id); setNameError("");
    if (!id) return;
    const revision = ++request.current;
    setMerchantLoading(true);
    try {
      const selected = (await api("/labels?kind=merchant&id=" + id)).items[0];
      if (request.current !== revision) return;
      setLogo(selected?.logo_data || ""); setLogoVersion(selected?.logo_version || 1);
      setCategory(selected?.category_id ?? null); setGroup(selected?.spending_group_id ?? null);
    } catch (error) {
      if (request.current === revision) notify((error as Error).message, true);
    } finally {
      if (request.current === revision) setMerchantLoading(false);
    }
  };
  const validatePattern = (value: string) => {
    const length = new TextEncoder().encode(value.trim()).length;
    if (length < 2) return "Use at least 2 characters.";
    if (length > 200) return "Use a shorter description match.";
    if (["|", "*", "(", ")", "+", "[", "{", "^", "$", String.fromCharCode(92)].some(symbol => value.includes(symbol))) {
      try { new RegExp(value, "i"); } catch { return "Fix the regex syntax (e.g. unmatched brackets)."; }
    }
    return "";
  };
  const save = () => run(async () => {
    let merchant = edit.merchant_id, version = logoVersion;
    if (!merchant && merchantName.trim()) {
      const created = await api("/labels", "POST", {
        kind: "merchant", account_id: Number(edit.account_id), name: merchantName,
      });
      merchant = created.id; version = created.logo_version;
      update("merchant_id", merchant); setLogoVersion(version);
    }
    if (!merchant) { setNameError("Choose a merchant or enter a new merchant name."); return; }
    const payload: Record<string, unknown> = {
      account_id: Number(edit.account_id), merchant_id: merchant, pattern: edit.pattern,
      direction: edit.direction, enabled: !!edit.enabled, version: edit.version || 0,
      priority: Number(edit.priority), category_id: category,
      spending_group_id: group, clear_category: category === null, clear_group: group === null,
    };
    if (logoDirty) Object.assign(payload, { merchant_logo: logo, merchant_version: version });
    await api("/merchant-rules" + (edit.id ? "/" + edit.id : ""), edit.id ? "PUT" : "POST", payload);
    refresh(); (onDone || onClose)();
  }, "Merchant rule saved", message => {
    if (message === "Choose an active category") { setCategoryError(message); return true; }
    if (message === "Choose an existing spending group") { setGroupError(message); return true; }
    return false;
  });
  return <Modal title={edit.id ? "Edit merchant rule" : "New merchant rule"} onClose={() => { if (!busy) onClose(); }}>
    <Form onSubmit={save}>
      <PagedSelect url="/accounts?role=editor" label="Scope" required value={edit.account_id}
        options={data.accounts.filter(a => a.role === "editor")}
        specialOptions={data.user.budget_member ? [{ value: "0", label: "All accounts" }] : []}
        hint="All accounts shares this rule across future imports."
        onChange={value => { update("account_id", value); update("merchant_id", null); clearMerchant(); }} />
      {edit.account_id !== "" && <ChoiceField label="Merchant" onError={message => notify(message, true)}
        options={[]} source={"/labels?kind=merchant&" + (Number(edit.account_id) === 0 ? "scope=global" : "account=" + edit.account_id)}
        value={edit.merchant_id} onChange={selectMerchant} />}
      <Field label="New merchant name" serverError={nameError} hint="Use this only if the merchant is not already listed.">
        <input required={!edit.merchant_id} minLength={2} maxLength={100} value={merchantName}
          onChange={event => { setMerchantName(event.target.value); setNameError(""); }} />
      </Field>
      <MerchantLogoField name={merchantName || edit.merchant_name || "Merchant"} value={logo}
        onChange={value => { setLogo(value); setLogoDirty(true); }} onBusy={setLogoBusy} />
      <Field label="Description contains" hint="Plain text or regex. Use | for alternatives, e.g. account fee|service fee."
        validate={validatePattern}>
        <input required minLength={2} maxLength={200} value={edit.pattern} onChange={event => update("pattern", event.target.value)} />
      </Field>
      <CategoryChoice label="Category for this rule" data={data} value={category} disabled={merchantLoading}
        validationError={categoryError} onChange={id => { setCategory(id); setCategoryError(""); }} refresh={refresh} notify={notify} />
      <ChoiceField label="Spending group for this rule" source="/spending-groups" options={[]} value={group}
        empty="No group" disabled={merchantLoading} validationError={groupError}
        onChange={id => { setGroup(id); setGroupError(""); }} onError={message => notify(message, true)} />
      <details className="details">
        <summary>More options{edit.direction !== "any" || Number(edit.priority) !== 0 || !edit.enabled ? " · Custom settings" : ""}</summary>
        <div className="form-grid">
          <Field label="Money direction"><select value={edit.direction} onChange={event => update("direction", event.target.value)}>
            <option value="any">Any direction</option><option value="debit">Money out</option><option value="credit">Money in</option>
          </select></Field>
          <Field label="Priority"><input type="number" min={-1000000} max={1000000} required value={edit.priority}
            onChange={event => update("priority", event.target.value)} /></Field>
        </div>
        <label className="check"><input type="checkbox" checked={!!edit.enabled} onChange={event => update("enabled", event.target.checked)} />Active</label>
      </details>
      <p className="footnote">One merchant can have several matching rules. Each rule saves its own category and spending group.
        Highest priority wins. Different classifications at the same priority leave the transaction in Needs review.
        Saving affects future imports; existing transactions keep their classifications.</p>
      <div className="editor-actions">
        <Button variant="primary" type="submit" loading={busy} disabled={logoBusy || merchantLoading}>Save rule</Button>
        <Button disabled={busy} onClick={onClose}>Cancel</Button>
      </div>
    </Form>
  </Modal>;
}
