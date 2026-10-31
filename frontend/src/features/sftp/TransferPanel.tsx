// Transferências da aba SFTP: progresso, velocidade e cancelamento (FR-020).
import { fmtBytes, t } from '../../i18n/pt-BR';
import { isFinished, useTransfers, type TransferView } from '../../stores/transfers';
import { Button } from '../../ui/Button';
import { CloseIcon, DownloadIcon, UploadIcon } from '../../ui/icons';
import s from './sftp.module.css';

const nameOf = (x: TransferView) =>
  (x.direction === 'upload' ? x.localPath.split(/[\\/]/) : x.remotePath.split('/')).pop() ?? '';

function Row({ x }: { x: TransferView }) {
  const cancel = useTransfers((st) => st.cancel);
  const pct = x.bytesTotal > 0 ? Math.min(100, (x.bytesDone / x.bytesTotal) * 100) : 0;
  const name = nameOf(x);
  return (
    <li className={s.transfer} data-state={x.state} aria-label={name}>
      {x.direction === 'upload' ? (
        <UploadIcon className={s.icon} aria-label={t.sftp.directions.upload} />
      ) : (
        <DownloadIcon className={s.icon} aria-label={t.sftp.directions.download} />
      )}
      <div className={s.transferMain}>
        <div className={s.transferLine}>
          <span className={s.name}>{name}</span>
          <span className={s.transferState}>
            {x.state === 'running' && x.speed > 0
              ? `${fmtBytes(x.speed)}/s`
              : (x.reason ?? t.sftp.states[x.state])}
          </span>
        </div>
        <div
          className={s.progress}
          role="progressbar"
          aria-label={name}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(pct)}
        >
          <div className={s.progressFill} style={{ width: `${String(pct)}%` }} />
        </div>
        <div className={s.transferBytes}>
          {fmtBytes(x.bytesDone)} / {fmtBytes(x.bytesTotal)}
        </div>
      </div>
      {!isFinished(x) && (
        <Button
          iconOnly
          variant="ghost"
          aria-label={`${t.sftp.cancelTransfer} ${name}`}
          title={t.sftp.cancelTransfer}
          onClick={() => {
            cancel(x.id);
          }}
        >
          <CloseIcon />
        </Button>
      )}
    </li>
  );
}

export function TransferPanel({ sessionId }: { sessionId: string }) {
  const all = useTransfers((st) => st.transfers);
  const clearFinished = useTransfers((st) => st.clearFinished);
  const mine = all.filter((x) => x.sessionId === sessionId);
  if (mine.length === 0) return null;
  return (
    <section className={s.transfers} aria-label={t.sftp.transfers}>
      <header className={s.transfersHeader}>
        <h3 className={s.transfersTitle}>{t.sftp.transfers}</h3>
        <Button
          variant="ghost"
          disabled={!mine.some(isFinished)}
          onClick={() => {
            clearFinished(sessionId);
          }}
        >
          {t.sftp.clearFinished}
        </Button>
      </header>
      <ul className={s.transferList}>
        {mine.map((x) => (
          <Row key={x.id} x={x} />
        ))}
      </ul>
    </section>
  );
}
