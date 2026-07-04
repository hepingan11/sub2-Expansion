import React, { FormEvent, useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  CheckCircle2,
  CircleDollarSign,
  LogOut,
  Pencil,
  Plus,
  Search,
  Settings2,
  ShieldCheck,
  Trash2,
  X
} from 'lucide-react';
import {
  batchImportCodes,
  clearToken,
  CodePayload,
  createCode,
  deleteCode,
  fetchCheckInSettings,
  fetchCodes,
  fetchStats,
  getToken,
  login,
  PrizeTierSetting,
  RedeemCode,
  RedeemCodeStatus,
  Stats,
  updateCheckInSettings,
  updateCode
} from './api';
import './styles.css';

const emptyStats: Stats = { total: 0, available: 0, assigned: 0, used: 0, voided: 0 };

const statusText: Record<RedeemCodeStatus, string> = {
  AVAILABLE: '未绑定',
  ASSIGNED: '已绑定',
  USED: '已使用',
  VOIDED: '已作废'
};

function App() {
  const [authed, setAuthed] = useState(Boolean(getToken()));

  return authed ? <Dashboard onLogout={() => setAuthed(false)} /> : <Login onLogin={() => setAuthed(true)} />;
}

function Login({ onLogin }: { onLogin: () => void }) {
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function submit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      await login(username, password);
      onLogin();
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="login-shell">
      <section className="login-panel">
        <div className="brand-mark">
          <ShieldCheck size={26} />
        </div>
        <h1>兑换码管理</h1>
        <p>登录后导入兑换码池，查看签到绑定和兑换状态。</p>
        <form onSubmit={submit} className="login-form">
          <label>
            管理员账号
            <input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" />
          </label>
          <label>
            管理员密码
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              placeholder=""
            />
          </label>
          {error && <div className="error-line">{error}</div>}
          <button type="submit" disabled={loading}>
            {loading ? '登录中...' : '登录'}
          </button>
        </form>
      </section>
    </main>
  );
}

function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [codes, setCodes] = useState<RedeemCode[]>([]);
  const [stats, setStats] = useState<Stats>(emptyStats);
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState('');
  const [page, setPage] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState<RedeemCode | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [dailyMaxUsers, setDailyMaxUsers] = useState(0);
  const [dailyMaxUsersDraft, setDailyMaxUsersDraft] = useState('');
  const [prizeTiers, setPrizeTiers] = useState<PrizeTierSetting[]>([]);
  const [prizeTierDrafts, setPrizeTierDrafts] = useState([{ amount: '1.00', probability: '100.00' }]);
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [settingsSaved, setSettingsSaved] = useState(false);

  async function load(nextPage = page) {
    setLoading(true);
    setError('');
    try {
      const [statsData, pageData, settingsData] = await Promise.all([
        fetchStats(),
        fetchCodes({ keyword, status, page: nextPage, size: 10 }),
        fetchCheckInSettings()
      ]);
      setStats(statsData);
      setCodes(pageData.content);
      setPage(pageData.number);
      setTotalPages(Math.max(pageData.totalPages, 1));
      setDailyMaxUsers(settingsData.dailyMaxUsers);
      setDailyMaxUsersDraft(String(settingsData.dailyMaxUsers));
      setPrizeTiers(settingsData.prizeTiers);
      setPrizeTierDrafts(toPrizeTierDrafts(settingsData.prizeTiers));
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load(0);
  }, []);

  const summary = useMemo(() => [
    { label: '总兑换码', value: stats.total, tone: 'ink' },
    { label: '未绑定', value: stats.available, tone: 'green' },
    { label: '已绑定', value: stats.assigned, tone: 'blue' },
    { label: '已使用', value: stats.used, tone: 'blue' },
    { label: '已作废', value: stats.voided, tone: 'red' }
  ], [stats]);

  function logout() {
    clearToken();
    onLogout();
  }

  async function remove(id: number) {
    if (!window.confirm('确认删除这条兑换码记录？')) {
      return;
    }
    await deleteCode(id);
    load(page);
  }

  async function saveCheckInSettings(event: FormEvent) {
    event.preventDefault();
    const nextDailyMaxUsers = Number(dailyMaxUsersDraft);
    if (!Number.isInteger(nextDailyMaxUsers) || nextDailyMaxUsers < 0) {
      setError('每日签到上限必须是大于等于 0 的整数');
      return;
    }

    const parsedPrizeTiers = parsePrizeTierDrafts(prizeTierDrafts);
    if (typeof parsedPrizeTiers === 'string') {
      setError(parsedPrizeTiers);
      return;
    }

    setSettingsSaving(true);
    setSettingsSaved(false);
    setError('');
    try {
      const settings = await updateCheckInSettings(nextDailyMaxUsers, parsedPrizeTiers);
      setDailyMaxUsers(settings.dailyMaxUsers);
      setDailyMaxUsersDraft(String(settings.dailyMaxUsers));
      setPrizeTiers(settings.prizeTiers);
      setPrizeTierDrafts(toPrizeTierDrafts(settings.prizeTiers));
      setSettingsSaved(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存设置失败');
    } finally {
      setSettingsSaving(false);
    }
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <span className="eyebrow">Daily Check-in Reward</span>
          <h1>兑换码管理系统</h1>
        </div>
        <button className="ghost-btn" onClick={logout} title="退出登录">
          <LogOut size={18} />
          退出
        </button>
      </header>

      <section className="summary-grid">
        {summary.map((item) => (
          <article key={item.label} className={`metric metric-${item.tone}`}>
            <span>{item.label}</span>
            <strong>{item.value}</strong>
          </article>
        ))}
      </section>

      <form className="settings-panel" onSubmit={saveCheckInSettings}>
        <div className="settings-panel-head">
          <div className="settings-title">
            <Settings2 size={18} />
            <span>签到设置</span>
          </div>
          <button className="ghost-btn" type="submit" disabled={settingsSaving || !settingsChanged(dailyMaxUsers, dailyMaxUsersDraft, prizeTiers, prizeTierDrafts)}>
            <CheckCircle2 size={17} />
            {settingsSaving ? '保存中...' : '保存'}
          </button>
          {settingsSaved && <span className="settings-saved">已保存</span>}
        </div>

        <div className="settings-grid">
          <label>
            每日签到上限
            <input
              type="number"
              min="0"
              step="1"
              value={dailyMaxUsersDraft}
              onChange={(event) => {
                setDailyMaxUsersDraft(event.target.value);
                setSettingsSaved(false);
              }}
            />
          </label>

          <div className="tier-editor">
            <div className="tier-editor-head">
              <span>兑换码金额概率</span>
              <button
                type="button"
                className="ghost-btn"
                onClick={() => {
                  setPrizeTierDrafts((current) => [...current, { amount: '1.00', probability: '1.00' }]);
                  setSettingsSaved(false);
                }}
              >
                <Plus size={17} />
                添加
              </button>
            </div>
            <div className="tier-list">
              {prizeTierDrafts.map((tier, index) => (
                <div className="tier-row" key={index}>
                  <label>
                    金额
                    <input
                      type="number"
                      min="0.01"
                      step="0.01"
                      value={tier.amount}
                      onChange={(event) => updatePrizeTierDraft(index, 'amount', event.target.value, setPrizeTierDrafts, setSettingsSaved)}
                    />
                  </label>
                  <label>
                    概率 %
                    <input
                      type="number"
                      min="0.01"
                      max="100"
                      step="0.01"
                      value={tier.probability}
                      onChange={(event) => updatePrizeTierDraft(index, 'probability', event.target.value, setPrizeTierDrafts, setSettingsSaved)}
                    />
                  </label>
                  <button
                    type="button"
                    className="icon-btn"
                    title="删除"
                    disabled={prizeTierDrafts.length <= 1}
                    onClick={() => {
                      setPrizeTierDrafts((current) => current.filter((_, currentIndex) => currentIndex !== index));
                      setSettingsSaved(false);
                    }}
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              ))}
            </div>
            <div className={`probability-total ${prizeTierTotal(prizeTierDrafts) === 100 ? 'is-valid' : ''}`}>
              合计 {prizeTierTotal(prizeTierDrafts).toFixed(2)}%
            </div>
          </div>
        </div>
      </form>

      <section className="toolbar">
        <div className="search-box">
          <Search size={18} />
          <input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            onKeyDown={(event) => event.key === 'Enter' && load(0)}
            placeholder="搜索兑换码或用户 ID"
          />
        </div>
        <select value={status} onChange={(event) => setStatus(event.target.value)}>
          <option value="">全部状态</option>
          <option value="AVAILABLE">未绑定</option>
          <option value="ASSIGNED">已绑定</option>
          <option value="USED">已使用</option>
          <option value="VOIDED">已作废</option>
        </select>
        <button className="ghost-btn" onClick={() => load(0)}>
          <Search size={17} />
          查询
        </button>
        <button
          className="primary-btn"
          onClick={() => {
            setImportOpen(true);
          }}
        >
          <Plus size={18} />
          批量导入
        </button>
      </section>

      {error && <div className="error-banner">{error}</div>}

      <section className="table-panel">
        <table>
          <thead>
            <tr>
              <th>兑换码</th>
              <th>用户 ID</th>
              <th>日期</th>
              <th>金额</th>
              <th>状态</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {codes.map((code) => (
              <tr key={code.id}>
                <td className="code-cell">{code.code}</td>
                <td>{code.userId || '-'}</td>
                <td>{code.signDate || '-'}</td>
                <td className="amount-cell">
                  <CircleDollarSign size={16} />
                  {Number(code.amount).toFixed(2)}
                </td>
                <td><span className={`status status-${code.status.toLowerCase()}`}>{statusText[code.status]}</span></td>
                <td>{formatDateTime(code.createdAt)}</td>
                <td>
                  <div className="row-actions">
                    <button title="编辑" onClick={() => { setEditing(code); setModalOpen(true); }}>
                      <Pencil size={16} />
                    </button>
                    <button title="删除" onClick={() => remove(code.id)}>
                      <Trash2 size={16} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {!loading && codes.length === 0 && (
              <tr>
                <td colSpan={7} className="empty-cell">暂无兑换码记录</td>
              </tr>
            )}
            {loading && (
              <tr>
                <td colSpan={7} className="empty-cell">加载中...</td>
              </tr>
            )}
          </tbody>
        </table>
      </section>

      <footer className="pager">
        <button disabled={page <= 0} onClick={() => load(page - 1)}>上一页</button>
        <span>{page + 1} / {totalPages}</span>
        <button disabled={page + 1 >= totalPages} onClick={() => load(page + 1)}>下一页</button>
      </footer>

      {modalOpen && (
        <CodeModal
          code={editing}
          onClose={() => setModalOpen(false)}
          onSaved={() => {
            setModalOpen(false);
            load(page);
          }}
        />
      )}

      {importOpen && (
        <ImportModal
          onClose={() => setImportOpen(false)}
          onImported={() => {
            setImportOpen(false);
            load(0);
          }}
        />
      )}
    </main>
  );
}

function CodeModal({ code, onClose, onSaved }: { code: RedeemCode | null; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState<CodePayload>({
    code: code?.code ?? '',
    userId: code?.userId ?? '',
    signDate: code?.signDate ?? '',
    amount: code ? String(code.amount) : '1.00',
    status: code?.status ?? 'AVAILABLE'
  });
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  function patch<K extends keyof CodePayload>(key: K, value: CodePayload[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError('');
    try {
      if (code) {
        await updateCode(code.id, form);
      } else {
        await createCode(form);
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="modal-backdrop">
      <form className="modal" onSubmit={submit}>
        <div className="modal-head">
          <div>
            <span className="eyebrow">{code ? 'Edit Code' : 'New Code'}</span>
            <h2>{code ? '编辑兑换码' : '新增兑换码'}</h2>
          </div>
          <button type="button" className="icon-btn" onClick={onClose} title="关闭">
            <X size={18} />
          </button>
        </div>

        <label>
          兑换码
          <input value={form.code} onChange={(event) => patch('code', event.target.value)} required />
        </label>
        <label>
          用户 ID
          <input value={form.userId ?? ''} onChange={(event) => patch('userId', event.target.value)} placeholder="未绑定时留空" />
        </label>
        <label>
          签到日期
          <input type="date" value={form.signDate ?? ''} onChange={(event) => patch('signDate', event.target.value)} />
        </label>
        <label>
          金额
          <input type="number" min="0.01" step="0.01" value={form.amount} onChange={(event) => patch('amount', event.target.value)} required />
        </label>
        <label>
          状态
          <select value={form.status} onChange={(event) => patch('status', event.target.value as RedeemCodeStatus)}>
            <option value="AVAILABLE">未绑定</option>
            <option value="ASSIGNED">已绑定</option>
            <option value="USED">已使用</option>
            <option value="VOIDED">已作废</option>
          </select>
        </label>

        {error && <div className="error-line">{error}</div>}
        <button className="primary-btn wide" type="submit" disabled={saving}>
          <CheckCircle2 size={18} />
          {saving ? '保存中...' : '保存'}
        </button>
      </form>
    </div>
  );
}

function ImportModal({ onClose, onImported }: { onClose: () => void; onImported: () => void }) {
  const [codesText, setCodesText] = useState('');
  const [amount, setAmount] = useState('1.00');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError('');
    try {
      const data = await batchImportCodes({ codesText, amount });
      window.alert(`解析 ${data.totalParsed} 个，成功导入 ${data.imported} 个，重复 ${data.duplicated} 个`);
      onImported();
    } catch (err) {
      setError(err instanceof Error ? err.message : '导入失败');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="modal-backdrop">
      <form className="modal" onSubmit={submit}>
        <div className="modal-head">
          <div>
            <span className="eyebrow">Batch Import</span>
            <h2>批量导入兑换码</h2>
          </div>
          <button type="button" className="icon-btn" onClick={onClose} title="关闭">
            <X size={18} />
          </button>
        </div>

        <label>
          金额
          <input type="number" min="0.01" step="0.01" value={amount} onChange={(event) => setAmount(event.target.value)} required />
        </label>
        <label>
          兑换码
          <textarea
            value={codesText}
            onChange={(event) => setCodesText(event.target.value)}
            rows={8}
            placeholder={'每行一个兑换码，也支持空格、逗号分隔'}
            required
          />
        </label>

        {error && <div className="error-line">{error}</div>}
        <button className="primary-btn wide" type="submit" disabled={saving}>
          <CheckCircle2 size={18} />
          {saving ? '导入中...' : '导入'}
        </button>
      </form>
    </div>
  );
}

function formatDateTime(value: string) {
  return value ? value.replace('T', ' ').slice(0, 19) : '-';
}

function toPrizeTierDrafts(prizeTiers: PrizeTierSetting[]) {
  const tiers = prizeTiers.length ? prizeTiers : [{ amount: 1, probability: 100 }];
  return tiers.map((tier) => ({
    amount: Number(tier.amount).toFixed(2),
    probability: Number(tier.probability).toFixed(2)
  }));
}

function updatePrizeTierDraft(
  index: number,
  key: 'amount' | 'probability',
  value: string,
  setPrizeTierDrafts: React.Dispatch<React.SetStateAction<{ amount: string; probability: string }[]>>,
  setSettingsSaved: React.Dispatch<React.SetStateAction<boolean>>
) {
  setPrizeTierDrafts((current) => current.map((tier, currentIndex) => (
    currentIndex === index ? { ...tier, [key]: value } : tier
  )));
  setSettingsSaved(false);
}

function parsePrizeTierDrafts(drafts: { amount: string; probability: string }[]): PrizeTierSetting[] | string {
  if (drafts.length === 0) {
    return '请至少配置一个兑换码金额概率';
  }

  const tiers = drafts.map((tier) => ({
    amount: Number(tier.amount),
    probability: Number(tier.probability)
  }));

  if (tiers.some((tier) => !Number.isFinite(tier.amount) || tier.amount <= 0)) {
    return '金额必须大于 0';
  }
  if (tiers.some((tier) => !Number.isFinite(tier.probability) || tier.probability <= 0 || tier.probability > 100)) {
    return '概率必须大于 0 且不超过 100';
  }
  if (Math.abs(tiers.reduce((total, tier) => total + tier.probability, 0) - 100) > 0.001) {
    return '所有金额概率之和必须等于 100%';
  }

  return tiers;
}

function prizeTierTotal(drafts: { amount: string; probability: string }[]) {
  return drafts.reduce((total, tier) => {
    const probability = Number(tier.probability);
    return Number.isFinite(probability) ? total + probability : total;
  }, 0);
}

function settingsChanged(
  dailyMaxUsers: number,
  dailyMaxUsersDraft: string,
  prizeTiers: PrizeTierSetting[],
  prizeTierDrafts: { amount: string; probability: string }[]
) {
  if (dailyMaxUsersDraft !== String(dailyMaxUsers)) {
    return true;
  }
  return JSON.stringify(toPrizeTierDrafts(prizeTiers)) !== JSON.stringify(prizeTierDrafts);
}

createRoot(document.getElementById('root')!).render(<App />);
