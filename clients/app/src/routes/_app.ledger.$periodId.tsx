import { useEffect, useMemo, useState } from 'react'
import { createFileRoute } from '@tanstack/react-router'
import { EntityActionBar } from '#/components/EntityActionBar'
import { ImportIncomeDialog } from '#/components/ImportIncomeDialog'
import { PageHeader } from '#/components/PageHeader'
import { SelectCheckbox } from '#/components/SelectCheckbox'
import type { LedgerDetail } from '#/data/api'
import { getLedger, moneyToNumber, updateLedgerBill } from '#/data/api'
import { formatCurrency, formatDateMMDDYYYY } from '#/data/format'
import { useAppStore } from '#/data/store'
import { useMultiSelect } from '#/hooks/useMultiSelect'

// A ledger's `date` is its cutoff/payday — income belongs to it if earned
// sometime in the month leading up to that cutoff, not by calendar month
// (ledger periods like "August P2"/"September P1" don't align to one).
function isWithinLedgerWindow(incomeIso: string, ledgerIso: string): boolean {
  const incomeDate = new Date(incomeIso)
  const ledgerDate = new Date(ledgerIso)
  const windowStart = new Date(ledgerDate)
  windowStart.setUTCMonth(windowStart.getUTCMonth() - 1)
  return incomeDate > windowStart && incomeDate <= ledgerDate
}

export const Route = createFileRoute('/_app/ledger/$periodId')({
  component: LedgerItemPage,
})

function LedgerItemPage() {
  const { periodId } = Route.useParams()
  const store = useAppStore()
  const billSelection = useMultiSelect()
  const incomeSelection = useMultiSelect()
  const [detail, setDetail] = useState<LedgerDetail | null>(null)
  const [loadError, setLoadError] = useState(false)
  const [importOpen, setImportOpen] = useState(false)

  function reload() {
    getLedger(periodId)
      .then(setDetail)
      .catch((err: unknown) => {
        console.error('failed to load ledger', err)
        setLoadError(true)
      })
  }

  useEffect(() => {
    void store.ensureBills()
    void store.ensureIncome()
  }, [])

  const importCandidates = useMemo(() => {
    if (!detail) return []
    return store.income
      .filter(
        (income) =>
          income.ledgerId === null &&
          isWithinLedgerWindow(income.date, detail.date),
      )
      .sort((a, b) => b.date.localeCompare(a.date))
  }, [store.income, detail])

  useEffect(() => {
    store.setActiveLedger(periodId)
    setDetail(null)
    setLoadError(false)
    reload()
    return () => store.setActiveLedger(null)
  }, [periodId])

  // EditModal's LedgerBill add/edit saves directly through the API (no
  // route-local state to update) — reload once the modal closes so this
  // page's local `detail` picks up the change.
  useEffect(() => {
    if (store.modal === null) reload()
  }, [store.modal])

  async function handleTogglePaid(ledgerBillId: string) {
    if (!detail) return
    const lb = detail.ledgerBills.find((b) => b.id === ledgerBillId)
    if (!lb) return
    try {
      await updateLedgerBill(periodId, lb.id, {
        billId: lb.billId,
        name: lb.name,
        amount: lb.amount,
        dueDay: lb.dueDay,
        isPayed: !lb.isPayed,
        notes: lb.notes,
      })
      reload()
    } catch (err) {
      console.error('failed to update ledger bill', err)
      store.showToast('error', "couldn't save that — try again")
    }
  }

  async function handleMarkAllPaid() {
    if (!detail) return
    try {
      await Promise.all(
        detail.ledgerBills
          .filter((lb) => !lb.isPayed)
          .map((lb) =>
            updateLedgerBill(periodId, lb.id, {
              billId: lb.billId,
              name: lb.name,
              amount: lb.amount,
              dueDay: lb.dueDay,
              isPayed: true,
              notes: lb.notes,
            }),
          ),
      )
      reload()
    } catch (err) {
      console.error('failed to mark all paid', err)
      store.showToast('error', "couldn't save that — try again")
    }
  }

  function handleEditBill() {
    if (!detail) return
    const [id] = billSelection.selectedIds
    const entry = detail.ledgerBills.find((lb) => lb.id === id)
    if (entry) {
      store.selectLedgerBill(entry)
      store.openLedgerBillModal('Edit')
    }
  }

  function handleDeleteBills() {
    const ids = billSelection.selectedIds
    store.requestDelete(ids.length, () => {
      void store.deleteLedgerBillEntries(periodId, ids).then(reload)
      billSelection.clear()
    })
  }

  function handleRemoveIncomes() {
    const ids = incomeSelection.selectedIds
    void store.attachIncomesToLedger(null, ids).then(reload)
    incomeSelection.clear()
  }

  async function handleImportConfirm(ids: string[]) {
    await store.attachIncomesToLedger(periodId, ids)
    setImportOpen(false)
    reload()
  }

  if (loadError) {
    return (
      <>
        <PageHeader kicker="ledger item" title="Not found" />
        <div style={{ padding: '20px 24px' }}>
          Couldn&apos;t load this ledger cycle.
        </div>
      </>
    )
  }

  if (!detail) return null

  // detail.income/expenses/net are stored totals from the legacy import,
  // disconnected from the incomes/ledgerBills actually attached — compute
  // the real figures instead so they stay correct as rows are
  // attached/removed here. Bank balance has no association to derive from
  // (it's an external fact), so it stays as the stored/manually-edited value.
  const incomeTotal = detail.incomes.reduce(
    (sum, income) => sum + moneyToNumber(income.amount),
    0,
  )
  const expensesTotal = detail.ledgerBills.reduce(
    (sum, lb) => sum + moneyToNumber(lb.amount),
    0,
  )
  const netTotal =
    moneyToNumber(detail.bankBalance) + incomeTotal - expensesTotal

  return (
    <>
      <PageHeader
        kicker="ledger item"
        title={`${detail.name ?? 'Ledger cycle'} · ${formatDateMMDDYYYY(detail.date)}`}
      />
      <div
        className="grid min-w-0 flex-1 grid-cols-1 content-start items-start gap-[18px] lg:grid-cols-[1.5fr_1fr]"
        style={{ padding: '20px 24px 8px' }}
      >
        <div
          className="card elev-sm overflow-hidden"
          style={{ background: '#1b1d2e', padding: 0, gap: 0 }}
        >
          <div
            className="flex flex-col items-start gap-[8px] sm:flex-row sm:items-center"
            style={{ padding: '13px 16px' }}
          >
            <span className="card-kicker mono">bills and expenses</span>
            <span
              className="mono"
              style={{ fontSize: 11, color: 'rgba(233,233,237,.4)' }}
            >
              click a row to toggle paid
            </span>
            <div className="flex flex-wrap gap-[8px] sm:ml-auto">
              <EntityActionBar
                selectedCount={billSelection.count}
                onAdd={() => store.openLedgerBillModal('Add')}
                onEdit={handleEditBill}
                onDelete={handleDeleteBills}
              />
            </div>
          </div>
          <table className="table">
            <thead>
              <tr>
                <th style={{ paddingLeft: 16, width: 36 }} />
                <th>Bill</th>
                <th style={{ textAlign: 'right' }}>Amount</th>
                <th className="hidden sm:table-cell">Due</th>
                <th style={{ textAlign: 'right', paddingRight: 16 }}>Paid</th>
              </tr>
            </thead>
            <tbody>
              {detail.ledgerBills.map((lb) => (
                <tr
                  key={lb.id}
                  className="mb-row"
                  onClick={() => void handleTogglePaid(lb.id)}
                  style={{
                    background: billSelection.isSelected(lb.id)
                      ? 'rgba(145,132,217,.16)'
                      : 'transparent',
                  }}
                >
                  <td style={{ paddingLeft: 16 }}>
                    <SelectCheckbox
                      checked={billSelection.isSelected(lb.id)}
                      onToggle={() => billSelection.toggle(lb.id)}
                    />
                  </td>
                  <td>{lb.bill?.name ?? lb.name}</td>
                  <td className="mono" style={{ textAlign: 'right' }}>
                    {formatCurrency(moneyToNumber(lb.amount))}
                  </td>
                  <td
                    className="mono hidden sm:table-cell"
                    style={{ color: 'rgba(233,233,237,.55)' }}
                  >
                    {lb.dueDay ?? '—'}
                  </td>
                  <td
                    className="mono"
                    style={{
                      textAlign: 'right',
                      paddingRight: 16,
                      color: lb.isPayed ? '#b5abfc' : 'rgba(233,233,237,.3)',
                    }}
                  >
                    {lb.isPayed ? '✓' : '·'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="flex gap-[8px]" style={{ padding: '12px 16px' }}>
            <button
              className="btn btn-ghost mono"
              style={{ fontSize: 12 }}
              onClick={() => void handleMarkAllPaid()}
            >
              Mark all paid
            </button>
          </div>
        </div>

        <div className="flex flex-col gap-[18px]">
          <div
            className="card elev-sm overflow-hidden"
            style={{ background: '#1b1d2e', padding: 0, gap: 0 }}
          >
            <div
              className="flex flex-col items-start gap-[8px] sm:flex-row sm:items-center"
              style={{ padding: '13px 16px' }}
            >
              <span className="card-kicker mono">incomes</span>
              <div className="flex flex-wrap gap-[8px] sm:ml-auto">
                <button
                  className="btn btn-primary mono"
                  style={{ fontSize: 12.5 }}
                  onClick={() => store.openIncomeModal('Add')}
                >
                  + Add
                </button>
                <button
                  className="btn btn-secondary mono"
                  style={{ fontSize: 12.5 }}
                  onClick={() => setImportOpen(true)}
                >
                  + Import
                </button>
                <button
                  className="btn btn-secondary mono"
                  style={{ fontSize: 12.5 }}
                  disabled={incomeSelection.count === 0}
                  onClick={handleRemoveIncomes}
                >
                  Remove
                </button>
              </div>
            </div>
            <table className="table">
              <thead>
                <tr>
                  <th style={{ paddingLeft: 16, width: 36 }} />
                  <th>Date</th>
                  <th style={{ textAlign: 'right', paddingRight: 16 }}>
                    Amount
                  </th>
                </tr>
              </thead>
              <tbody>
                {detail.incomes.map((income) => (
                  <tr
                    key={income.id}
                    className="mb-row"
                    onClick={() => incomeSelection.toggle(income.id)}
                    style={{
                      background: incomeSelection.isSelected(income.id)
                        ? 'rgba(145,132,217,.16)'
                        : 'transparent',
                    }}
                  >
                    <td style={{ paddingLeft: 16 }}>
                      <SelectCheckbox
                        checked={incomeSelection.isSelected(income.id)}
                        onToggle={() => incomeSelection.toggle(income.id)}
                      />
                    </td>
                    <td className="mono">{formatDateMMDDYYYY(income.date)}</td>
                    <td
                      className="mono"
                      style={{ textAlign: 'right', paddingRight: 16 }}
                    >
                      {formatCurrency(moneyToNumber(income.amount))}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div
            className="card elev-sm"
            style={{ background: '#1b1d2e', padding: '16px 18px', gap: 11 }}
          >
            <div className="flex items-center gap-[8px]">
              <span className="card-kicker mono">summary</span>
              <button
                className="btn btn-secondary mono ml-auto"
                style={{ fontSize: 12 }}
                onClick={() => {
                  store.selectLedger(detail.id)
                  store.openLedgerModal('Edit')
                }}
              >
                Edit
              </button>
            </div>
            <div
              className="mono flex flex-col gap-[6px]"
              style={{ fontSize: 13 }}
            >
              <div className="flex">
                <span
                  className="flex-1"
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 4,
                    color: 'rgba(233,233,237,.6)',
                  }}
                >
                  Cashflow
                  <span
                    className="group relative inline-flex"
                    style={{ alignItems: 'center', cursor: 'help' }}
                  >
                    <span
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        width: 14,
                        height: 14,
                        borderRadius: '50%',
                        border: '1px solid rgba(233,233,237,.4)',
                        fontSize: 9,
                        lineHeight: 1,
                      }}
                    >
                      i
                    </span>
                    <span
                      className="invisible group-hover:visible absolute"
                      style={{
                        bottom: '130%',
                        left: '50%',
                        transform: 'translateX(-50%)',
                        width: 180,
                        padding: '6px 8px',
                        borderRadius: 6,
                        background: '#232532',
                        color: '#e9e9ed',
                        fontSize: 11,
                        lineHeight: 1.4,
                        textAlign: 'center',
                        boxShadow: '0 0 0 1px #3f424d, 0 4px 12px rgba(0,0,0,.4)',
                        zIndex: 10,
                      }}
                    >
                      whatever other cashflow you got outside of income
                    </span>
                  </span>
                </span>
                <span>{formatCurrency(moneyToNumber(detail.bankBalance))}</span>
              </div>
              <div className="flex">
                <span
                  className="flex-1"
                  style={{ color: 'rgba(233,233,237,.6)' }}
                >
                  Income ({detail.incomes.length} items)
                </span>
                <span>{formatCurrency(incomeTotal)}</span>
              </div>
              <div className="flex">
                <span
                  className="flex-1"
                  style={{ color: 'rgba(233,233,237,.6)' }}
                >
                  Expenses
                </span>
                <span>{formatCurrency(expensesTotal)}</span>
              </div>
            </div>
            <div
              className="h-px"
              style={{
                background:
                  'linear-gradient(to right,transparent,rgba(145,132,217,.5),transparent)',
              }}
            />
            <div className="flex items-baseline">
              <span
                className="mono flex-1"
                style={{
                  fontSize: 11,
                  letterSpacing: '.16em',
                  textTransform: 'uppercase',
                  color: '#9184d9',
                }}
              >
                net
              </span>
              <span className="mono" style={{ fontSize: 24 }}>
                {formatCurrency(netTotal)}
              </span>
            </div>
            {detail.notes && (
              <>
                <div
                  className="h-px"
                  style={{
                    background:
                      'linear-gradient(to right,transparent,rgba(145,132,217,.5),transparent)',
                  }}
                />
                <div className="flex flex-col gap-[4px]">
                  <span
                    className="mono"
                    style={{
                      fontSize: 11,
                      letterSpacing: '.16em',
                      textTransform: 'uppercase',
                      color: 'rgba(233,233,237,.4)',
                    }}
                  >
                    notes
                  </span>
                  <span style={{ fontSize: 13, lineHeight: 1.5 }}>
                    {detail.notes}
                  </span>
                </div>
              </>
            )}
          </div>
        </div>
      </div>
      <ImportIncomeDialog
        open={importOpen}
        candidates={importCandidates}
        onClose={() => setImportOpen(false)}
        onConfirm={handleImportConfirm}
      />
    </>
  )
}
