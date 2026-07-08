import { useEffect, useState } from 'react';
import { AlertCircle, CheckCircle2, Info, X } from 'lucide-react';

type NoticeType = 'error' | 'success' | 'info';

interface NoticePayload {
  type: NoticeType;
  title?: string;
  message: string;
}

interface Notice extends NoticePayload {
  id: number;
}

interface DialogPayload {
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  resolve: (confirmed: boolean) => void;
}

let noticeId = 1;

const noticeEvent = 'app:notice';
const dialogEvent = 'app:dialog';

export function notifyError(message: string, title = '错误') {
  notify({ type: 'error', title, message });
}

export function notifySuccess(message: string, title = '成功') {
  notify({ type: 'success', title, message });
}

export function notifyInfo(message: string, title = '提示') {
  notify({ type: 'info', title, message });
}

export function alertDialog(options: Omit<DialogPayload, 'resolve' | 'cancelText'>) {
  return confirmDialog({ ...options, cancelText: '' });
}

export function confirmDialog(options: Omit<DialogPayload, 'resolve'>) {
  return new Promise<boolean>((resolve) => {
    window.dispatchEvent(new CustomEvent<DialogPayload>(dialogEvent, {
      detail: { ...options, resolve }
    }));
  });
}

function notify(payload: NoticePayload) {
  if (!payload.message) return;
  window.dispatchEvent(new CustomEvent<NoticePayload>(noticeEvent, { detail: payload }));
}

export function FeedbackHost() {
  const [notices, setNotices] = useState<Notice[]>([]);
  const [dialog, setDialog] = useState<DialogPayload | null>(null);

  useEffect(() => {
    function onNotice(event: Event) {
      const payload = (event as CustomEvent<NoticePayload>).detail;
      const id = noticeId++;
      setNotices((current) => [...current, { ...payload, id }]);
      window.setTimeout(() => {
        setNotices((current) => current.filter((notice) => notice.id !== id));
      }, payload.type === 'error' ? 4500 : 3000);
    }

    function onDialog(event: Event) {
      setDialog((event as CustomEvent<DialogPayload>).detail);
    }

    window.addEventListener(noticeEvent, onNotice);
    window.addEventListener(dialogEvent, onDialog);
    return () => {
      window.removeEventListener(noticeEvent, onNotice);
      window.removeEventListener(dialogEvent, onDialog);
    };
  }, []);

  function closeNotice(id: number) {
    setNotices((current) => current.filter((notice) => notice.id !== id));
  }

  function resolveDialog(confirmed: boolean) {
    dialog?.resolve(confirmed);
    setDialog(null);
  }

  return (
    <>
      <div className="el-message-stack" aria-live="polite">
        {notices.map((notice) => (
          <div className={`el-message-toast is-${notice.type}`} key={notice.id}>
            <span className="el-message-icon">
              {notice.type === 'error' ? <AlertCircle size={18} /> : notice.type === 'success' ? <CheckCircle2 size={18} /> : <Info size={18} />}
            </span>
            <div>
              {notice.title && <strong>{notice.title}</strong>}
              <p>{notice.message}</p>
            </div>
            <button className="el-message-close" type="button" onClick={() => closeNotice(notice.id)} title="关闭">
              <X size={15} />
            </button>
          </div>
        ))}
      </div>

      {dialog && (
        <div className="el-dialog-backdrop">
          <section className="el-dialog" role="dialog" aria-modal="true" aria-label={dialog.title}>
            <div className="el-dialog-head">
              <strong>{dialog.title}</strong>
              <button type="button" className="icon-btn" onClick={() => resolveDialog(false)} title="关闭">
                <X size={17} />
              </button>
            </div>
            <p>{dialog.message}</p>
            <div className="el-dialog-actions">
              {dialog.cancelText !== '' && (
                <button type="button" className="ghost-btn" onClick={() => resolveDialog(false)}>
                  {dialog.cancelText || '取消'}
                </button>
              )}
              <button type="button" className={dialog.danger ? 'danger-btn' : 'primary-btn'} onClick={() => resolveDialog(true)}>
                {dialog.confirmText || '确定'}
              </button>
            </div>
          </section>
        </div>
      )}
    </>
  );
}
