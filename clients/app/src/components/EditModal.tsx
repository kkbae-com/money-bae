import { useEffect, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Mascot } from '#/components/Mascot'
import type {
  BillInput,
  HolidayHourInput,
  IncomeInput,
  LedgerBillInput,
  LedgerInput,
  PtoInput,
  PtoPlanInput,
  PtoPlanStatus,
} from '#/data/api'
import { moneyToNumber, numberToMoney } from '#/data/api'
import {
  formatDateInputValue,
  formatDateMMDDYYYY,
  parseDateInputValue,
} from '#/data/format'
import { useAppStore } from '#/data/store'

const MODAL_NOUN = {
  bill: 'bill',
  income: 'income entry',
  ledger: 'ledger cycle',
  ledgerBill: 'bill/expense to cycle',
  pto: 'PTO entry',
  ptoYear: 'PTO year',
  holiday: 'holiday',
} as const

// Sentinel value for the "Generic expense" option in the Bill select — a
// one-off expense scoped to just this ledger, with no catalog Bill behind
// it. Distinct from '' (the disabled "Select a bill…" placeholder).
const GENERIC_EXPENSE_VALUE = '__generic__'

export function EditModal() {
  const store = useAppStore()

  const [incomeForm, setIncomeForm] = useState({
    date: '',
    amount: '',
    notes: '',
  })
  const [billForm, setBillForm] = useState({
    name: '',
    amount: '',
    dueDay: '',
    isAutoPay: false,
    notes: '',
  })
  const [ledgerForm, setLedgerForm] = useState({
    date: '',
    name: '',
    bankBalance: '',
    notes: '',
  })
  const [ledgerBillForm, setLedgerBillForm] = useState({
    billId: '',
    name: '',
    amount: '',
    dueDay: '',
    isPayed: false,
    notes: '',
  })
  const [ptoYearForm, setPtoYearForm] = useState({
    year: '',
    availableHours: '',
    prevYearHours: '0',
    rolloverHours: false,
  })
  const [ptoPlanForm, setPtoPlanForm] = useState({
    startDate: '',
    endDate: '',
    name: '',
    description: '',
    hours: '',
    status: 'Planned' as PtoPlanStatus,
  })
  const [holidayForm, setHolidayForm] = useState({
    date: '',
    name: '',
    hours: '',
  })

  useEffect(() => {
    if (store.modal !== 'income') return
    void store.loadLedgers() // so an assigned income can link to its ledger by name
    const existing =
      store.modalMode === 'Edit'
        ? store.income.find((i) => i.id === store.incomeSelected)
        : undefined
    setIncomeForm(
      existing
        ? {
            date: formatDateInputValue(existing.date),
            amount: moneyToNumber(existing.amount).toFixed(2),
            notes: existing.notes ?? '',
          }
        : {
            date: formatDateInputValue(new Date().toISOString()),
            amount: '',
            notes: '',
          },
    )
  }, [store.modal, store.modalMode, store.incomeSelected, store.income])

  useEffect(() => {
    if (store.modal !== 'bill') return
    const existing =
      store.modalMode === 'Edit'
        ? store.bills.find((b) => b.id === store.billSelected)
        : undefined
    setBillForm(
      existing
        ? {
            name: existing.name,
            amount: moneyToNumber(existing.amount).toFixed(2),
            dueDay: existing.dueDay != null ? String(existing.dueDay) : '',
            isAutoPay: existing.isAutoPay,
            notes: existing.notes ?? '',
          }
        : { name: '', amount: '', dueDay: '', isAutoPay: false, notes: '' },
    )
  }, [store.modal, store.modalMode, store.billSelected, store.bills])

  useEffect(() => {
    if (store.modal !== 'ledger') return
    const existing =
      store.modalMode === 'Edit'
        ? store.ledgers.find((l) => l.id === store.ledgerSelected)
        : undefined
    setLedgerForm(
      existing
        ? {
            date: formatDateInputValue(existing.date),
            name: existing.name ?? '',
            bankBalance: moneyToNumber(existing.bankBalance).toFixed(2),
            notes: existing.notes ?? '',
          }
        : { date: '', name: '', bankBalance: '', notes: '' },
    )
  }, [store.modal, store.modalMode, store.ledgerSelected, store.ledgers])

  useEffect(() => {
    if (store.modal !== 'ledgerBill') return
    const existing =
      store.modalMode === 'Edit' ? store.selectedLedgerBill : null
    setLedgerBillForm(
      existing
        ? {
            billId: existing.billId ?? GENERIC_EXPENSE_VALUE,
            name: existing.name ?? '',
            amount: moneyToNumber(existing.amount).toFixed(2),
            dueDay: existing.dueDay != null ? String(existing.dueDay) : '',
            isPayed: existing.isPayed,
            notes: existing.notes ?? '',
          }
        : {
            billId: '',
            name: '',
            amount: '',
            dueDay: '',
            isPayed: false,
            notes: '',
          },
    )
  }, [store.modal, store.modalMode, store.selectedLedgerBill])

  useEffect(() => {
    if (store.modal !== 'ptoYear') return
    const existing =
      store.modalMode === 'Edit'
        ? store.ptos.find((p) => p.id === store.ptoYearSelected)
        : undefined
    setPtoYearForm(
      existing
        ? {
            year: String(existing.year),
            availableHours: existing.availableHours,
            prevYearHours: existing.prevYearHours,
            rolloverHours: existing.rolloverHours,
          }
        : {
            year: '',
            availableHours: '',
            prevYearHours: '0',
            rolloverHours: false,
          },
    )
  }, [store.modal, store.modalMode, store.ptoYearSelected, store.ptos])

  useEffect(() => {
    if (store.modal !== 'pto') return
    const existing = store.modalMode === 'Edit' ? store.selectedPtoPlan : null
    setPtoPlanForm(
      existing
        ? {
            startDate: formatDateInputValue(existing.startDate),
            endDate: formatDateInputValue(existing.endDate),
            name: existing.name,
            description: existing.description ?? '',
            hours: existing.hours,
            status: existing.status,
          }
        : {
            startDate: '',
            endDate: '',
            name: '',
            description: '',
            hours: '',
            status: 'Planned',
          },
    )
  }, [store.modal, store.modalMode, store.selectedPtoPlan])

  useEffect(() => {
    if (store.modal !== 'holiday') return
    const existing = store.modalMode === 'Edit' ? store.selectedHoliday : null
    setHolidayForm(
      existing
        ? {
            date: formatDateInputValue(existing.date),
            name: existing.name,
            hours: existing.hours,
          }
        : { date: '', name: '', hours: '' },
    )
  }, [store.modal, store.modalMode, store.selectedHoliday])

  const editingIncome =
    store.modal === 'income' && store.modalMode === 'Edit'
      ? store.income.find((i) => i.id === store.incomeSelected)
      : undefined
  const editingIncomeLedger = editingIncome?.ledgerId
    ? store.ledgers.find((l) => l.id === editingIncome.ledgerId)
    : undefined

  if (!store.modal) return null

  const title = `${store.modalMode} ${MODAL_NOUN[store.modal]}`

  async function handleSave() {
    if (store.modal === 'income') {
      const existingIncome =
        store.modalMode === 'Edit'
          ? store.income.find((i) => i.id === store.incomeSelected)
          : undefined
      const input: IncomeInput = {
        date: parseDateInputValue(incomeForm.date),
        amount: numberToMoney(Number(incomeForm.amount) || 0),
        ledgerId:
          store.modalMode === 'Add'
            ? store.activeLedgerId
            : (existingIncome?.ledgerId ?? null),
        notes: incomeForm.notes || null,
      }
      const ok =
        store.modalMode === 'Add'
          ? await store.addIncomeEntry(input)
          : await store.editIncomeEntry(input)
      if (!ok) return
    } else if (store.modal === 'bill') {
      const input: BillInput = {
        name: billForm.name,
        amount: numberToMoney(Number(billForm.amount) || 0),
        dueDay: billForm.dueDay ? Number(billForm.dueDay) : null,
        isAutoPay: billForm.isAutoPay,
        notes: billForm.notes || null,
      }
      const ok =
        store.modalMode === 'Add'
          ? await store.addBillEntry(input)
          : await store.editBillEntry(input)
      if (!ok) return
    } else if (store.modal === 'ledger') {
      // income/expenses aren't editable here (the ledger detail page
      // computes them live from attached incomes/bills) — pass the existing
      // values straight through so saving date/name/bankBalance/notes
      // doesn't wipe them via the API's full-overwrite PUT. net/total are
      // entirely server-computed now (see servers/api's resolveNet/
      // resolveTotal), so there's nothing to pass through for those.
      const existingLedger =
        store.modalMode === 'Edit'
          ? store.ledgers.find((l) => l.id === store.ledgerSelected)
          : undefined
      const input: LedgerInput = {
        date: parseDateInputValue(ledgerForm.date),
        name: ledgerForm.name || null,
        bankBalance: numberToMoney(Number(ledgerForm.bankBalance) || 0),
        income: existingLedger?.income,
        expenses: existingLedger?.expenses,
        notes: ledgerForm.notes || null,
      }
      const ok =
        store.modalMode === 'Add'
          ? await store.addLedgerEntry(input)
          : await store.editLedgerEntry(input)
      if (!ok) return
    } else if (store.modal === 'ledgerBill') {
      const isGenericExpense = ledgerBillForm.billId === GENERIC_EXPENSE_VALUE
      const input: LedgerBillInput = {
        billId: isGenericExpense ? null : ledgerBillForm.billId || null,
        name: isGenericExpense ? ledgerBillForm.name : null,
        amount: numberToMoney(Number(ledgerBillForm.amount) || 0),
        dueDay: ledgerBillForm.dueDay ? Number(ledgerBillForm.dueDay) : null,
        isPayed: ledgerBillForm.isPayed,
        notes: ledgerBillForm.notes || null,
      }
      if (!input.billId && !input.name) return
      const ok =
        store.modalMode === 'Add'
          ? await store.addLedgerBillEntry(input)
          : await store.editLedgerBillEntry(input)
      if (!ok) return
    } else if (store.modal === 'ptoYear') {
      const input: PtoInput = {
        year: Number(ptoYearForm.year) || 0,
        prevYearHours: ptoYearForm.prevYearHours || '0',
        availableHours: ptoYearForm.availableHours || '0',
        rolloverHours: ptoYearForm.rolloverHours,
      }
      const ok =
        store.modalMode === 'Add'
          ? await store.addPtoYearEntry(input)
          : await store.editPtoYearEntry(input)
      if (!ok) return
    } else if (store.modal === 'pto') {
      const input: PtoPlanInput = {
        startDate: parseDateInputValue(ptoPlanForm.startDate),
        endDate: parseDateInputValue(ptoPlanForm.endDate),
        name: ptoPlanForm.name,
        description: ptoPlanForm.description || null,
        hours: ptoPlanForm.hours || '0',
        status: ptoPlanForm.status,
        customHours: false,
      }
      const ok =
        store.modalMode === 'Add'
          ? await store.addPtoPlanEntry(input)
          : await store.editPtoPlanEntry(input)
      if (!ok) return
    } else if (store.modal === 'holiday') {
      const input: HolidayHourInput = {
        date: parseDateInputValue(holidayForm.date),
        name: holidayForm.name,
        hours: holidayForm.hours || '0',
      }
      const ok =
        store.modalMode === 'Add'
          ? await store.addHolidayEntry(input)
          : await store.editHolidayEntry(input)
      if (!ok) return
    }
    store.closeModal()
  }

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center p-4"
      style={{ background: 'rgba(16,17,32,.62)' }}
    >
      <div
        className="dialog elev-lg relative"
        style={{ background: '#1b1d2e', width: 'min(430px, 100%)' }}
      >
        <div
          className="absolute -top-[30px] right-[14px]"
          style={{ width: 56 }}
        >
          <Mascot size={56} />
        </div>
        <div className="dialog-title mono" style={{ fontSize: 17 }}>
          {title}
        </div>

        {store.modal === 'bill' && (
          <div className="flex flex-col gap-[11px]">
            <div className="field">
              <label>Name</label>
              <input
                className="input mono"
                value={billForm.name}
                onChange={(e) =>
                  setBillForm((f) => ({ ...f, name: e.target.value }))
                }
              />
            </div>
            <div className="flex flex-col gap-[11px] sm:flex-row">
              <div className="field sm:flex-1">
                <label>Amount</label>
                <input
                  className="input mono"
                  type="number"
                  step="0.01"
                  value={billForm.amount}
                  onChange={(e) =>
                    setBillForm((f) => ({ ...f, amount: e.target.value }))
                  }
                />
              </div>
              <div className="field sm:w-[120px]">
                <label>Due day (1–31)</label>
                <input
                  className="input mono"
                  type="number"
                  min={1}
                  max={31}
                  value={billForm.dueDay}
                  onChange={(e) =>
                    setBillForm((f) => ({ ...f, dueDay: e.target.value }))
                  }
                />
              </div>
            </div>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={billForm.isAutoPay}
                onChange={(e) =>
                  setBillForm((f) => ({ ...f, isAutoPay: e.target.checked }))
                }
              />
              <span className="box" />
              <span>Auto pay</span>
            </label>
            <div className="field">
              <label>Notes</label>
              <textarea
                className="input"
                style={{ minHeight: 66 }}
                value={billForm.notes}
                onChange={(e) =>
                  setBillForm((f) => ({ ...f, notes: e.target.value }))
                }
              />
            </div>
          </div>
        )}

        {store.modal === 'income' && (
          <div className="flex flex-col gap-[11px]">
            <div className="flex flex-col gap-[11px] sm:flex-row">
              <div className="field sm:flex-1">
                <label>Date</label>
                <input
                  className="input mono"
                  type="date"
                  value={incomeForm.date}
                  onChange={(e) =>
                    setIncomeForm((f) => ({ ...f, date: e.target.value }))
                  }
                />
              </div>
              <div className="field sm:w-[140px]">
                <label>Amount</label>
                <input
                  className="input mono"
                  type="number"
                  step="0.01"
                  value={incomeForm.amount}
                  onChange={(e) =>
                    setIncomeForm((f) => ({ ...f, amount: e.target.value }))
                  }
                  placeholder="1842.60"
                />
              </div>
            </div>
            <div className="field">
              <label>Notes</label>
              <input
                className="input"
                value={incomeForm.notes}
                onChange={(e) =>
                  setIncomeForm((f) => ({ ...f, notes: e.target.value }))
                }
                placeholder="overtime, bonus, …"
              />
            </div>
            {editingIncomeLedger && (
              <div className="field">
                <label>Ledger</label>
                <Link
                  className="mono"
                  to="/ledger/$periodId"
                  params={{ periodId: editingIncomeLedger.id }}
                  onClick={store.closeModal}
                  style={{ color: '#9184d9' }}
                >
                  {editingIncomeLedger.name ??
                    formatDateMMDDYYYY(editingIncomeLedger.date)}
                </Link>
              </div>
            )}
          </div>
        )}

        {store.modal === 'ledger' && (
          <div className="flex flex-col gap-[11px]">
            <div className="field">
              <label>Date</label>
              <input
                className="input mono"
                type="date"
                value={ledgerForm.date}
                onChange={(e) =>
                  setLedgerForm((f) => ({ ...f, date: e.target.value }))
                }
              />
            </div>
            <div className="field">
              <label>Cycle name</label>
              <input
                className="input mono"
                value={ledgerForm.name}
                onChange={(e) =>
                  setLedgerForm((f) => ({ ...f, name: e.target.value }))
                }
                placeholder="December P1"
              />
            </div>
            <div className="field">
              <label style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
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
                      color: 'rgba(233,233,237,.6)',
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
              </label>
              <input
                className="input mono"
                type="number"
                step="0.01"
                value={ledgerForm.bankBalance}
                onChange={(e) =>
                  setLedgerForm((f) => ({
                    ...f,
                    bankBalance: e.target.value,
                  }))
                }
                placeholder="1842.60"
              />
            </div>
            <div className="field">
              <label>Notes</label>
              <input
                className="input"
                value={ledgerForm.notes}
                onChange={(e) =>
                  setLedgerForm((f) => ({ ...f, notes: e.target.value }))
                }
                placeholder="…"
              />
            </div>
          </div>
        )}

        {store.modal === 'ledgerBill' && (
          <div className="flex flex-col gap-[11px]">
            <div className="field">
              <label>Bill</label>
              {store.modalMode === 'Edit' ? (
                // Which bill/expense an entry is tied to can't change after
                // creation — a disabled <select> still looks interactive
                // (and is easy to misread), so show the resolved name as
                // plain locked text instead.
                <input
                  className="input mono"
                  value={
                    store.selectedLedgerBill?.bill?.name ??
                    store.selectedLedgerBill?.name ??
                    ''
                  }
                  disabled
                />
              ) : (
                <select
                  className="input mono"
                  value={ledgerBillForm.billId}
                  onChange={(e) => {
                    const billId = e.target.value
                    if (billId === GENERIC_EXPENSE_VALUE) {
                      setLedgerBillForm((f) => ({ ...f, billId }))
                      return
                    }
                    const bill = store.bills.find((b) => b.id === billId)
                    setLedgerBillForm((f) => ({
                      ...f,
                      billId,
                      amount: bill
                        ? moneyToNumber(bill.amount).toFixed(2)
                        : f.amount,
                      dueDay:
                        bill?.dueDay != null ? String(bill.dueDay) : f.dueDay,
                    }))
                  }}
                >
                  <option value="" disabled>
                    Select a bill…
                  </option>
                  <option value={GENERIC_EXPENSE_VALUE}>
                    Generic Expense
                  </option>
                  {store.bills.map((bill) => (
                    <option key={bill.id} value={bill.id}>
                      {bill.name}
                    </option>
                  ))}
                </select>
              )}
            </div>
            {ledgerBillForm.billId === GENERIC_EXPENSE_VALUE && (
              <div className="field">
                <label>Name</label>
                <input
                  className="input"
                  value={ledgerBillForm.name}
                  onChange={(e) =>
                    setLedgerBillForm((f) => ({
                      ...f,
                      name: e.target.value,
                    }))
                  }
                />
              </div>
            )}
            <div className="flex flex-col gap-[11px] sm:flex-row">
              <div className="field sm:flex-1">
                <label>Amount</label>
                <input
                  className="input mono"
                  type="number"
                  step="0.01"
                  value={ledgerBillForm.amount}
                  onChange={(e) =>
                    setLedgerBillForm((f) => ({
                      ...f,
                      amount: e.target.value,
                    }))
                  }
                />
              </div>
              <div className="field sm:w-[120px]">
                <label>Due day (1–31)</label>
                <input
                  className="input mono"
                  type="number"
                  min={1}
                  max={31}
                  value={ledgerBillForm.dueDay}
                  onChange={(e) =>
                    setLedgerBillForm((f) => ({
                      ...f,
                      dueDay: e.target.value,
                    }))
                  }
                />
              </div>
            </div>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={ledgerBillForm.isPayed}
                onChange={(e) =>
                  setLedgerBillForm((f) => ({
                    ...f,
                    isPayed: e.target.checked,
                  }))
                }
              />
              <span className="box" />
              <span>Paid</span>
            </label>
            <div className="field">
              <label>Notes</label>
              <textarea
                className="input"
                style={{ minHeight: 66 }}
                value={ledgerBillForm.notes}
                onChange={(e) =>
                  setLedgerBillForm((f) => ({ ...f, notes: e.target.value }))
                }
              />
            </div>
          </div>
        )}

        {store.modal === 'ptoYear' && (
          <div className="flex flex-col gap-[11px]">
            <div className="flex flex-col gap-[11px] sm:flex-row">
              <div className="field sm:flex-1">
                <label>Year</label>
                <input
                  className="input mono"
                  type="number"
                  value={ptoYearForm.year}
                  onChange={(e) =>
                    setPtoYearForm((f) => ({ ...f, year: e.target.value }))
                  }
                  placeholder="2027"
                />
              </div>
              <div className="field sm:flex-1">
                <label>Available (hours)</label>
                <input
                  className="input mono"
                  type="number"
                  step="0.01"
                  value={ptoYearForm.availableHours}
                  onChange={(e) =>
                    setPtoYearForm((f) => ({
                      ...f,
                      availableHours: e.target.value,
                    }))
                  }
                  placeholder="200.00"
                />
              </div>
            </div>
          </div>
        )}

        {store.modal === 'pto' && (
          <div className="flex flex-col gap-[11px]">
            <div className="field">
              <label>Name</label>
              <input
                className="input mono"
                value={ptoPlanForm.name}
                onChange={(e) =>
                  setPtoPlanForm((f) => ({ ...f, name: e.target.value }))
                }
                placeholder="Christmas"
              />
            </div>
            <div className="flex flex-col gap-[11px] sm:flex-row">
              <div className="field sm:flex-1">
                <label>Start</label>
                <input
                  className="input mono"
                  type="date"
                  value={ptoPlanForm.startDate}
                  onChange={(e) =>
                    setPtoPlanForm((f) => ({
                      ...f,
                      startDate: e.target.value,
                    }))
                  }
                />
              </div>
              <div className="field sm:flex-1">
                <label>End</label>
                <input
                  className="input mono"
                  type="date"
                  value={ptoPlanForm.endDate}
                  onChange={(e) =>
                    setPtoPlanForm((f) => ({ ...f, endDate: e.target.value }))
                  }
                />
              </div>
            </div>
            <div className="flex flex-col gap-[11px] sm:flex-row">
              <div className="field sm:w-[120px]">
                <label>Hours</label>
                <input
                  className="input mono"
                  type="number"
                  step="0.01"
                  value={ptoPlanForm.hours}
                  onChange={(e) =>
                    setPtoPlanForm((f) => ({ ...f, hours: e.target.value }))
                  }
                  placeholder="72.00"
                />
              </div>
              <div className="field sm:flex-1">
                <label>Status</label>
                <div className="seg w-full">
                  <label className="seg-opt flex-1 justify-center">
                    <input
                      type="radio"
                      name="ptostatus"
                      checked={ptoPlanForm.status === 'Planned'}
                      onChange={() =>
                        setPtoPlanForm((f) => ({ ...f, status: 'Planned' }))
                      }
                    />
                    Planned
                  </label>
                  <label className="seg-opt flex-1 justify-center">
                    <input
                      type="radio"
                      name="ptostatus"
                      checked={ptoPlanForm.status === 'Completed'}
                      onChange={() =>
                        setPtoPlanForm((f) => ({ ...f, status: 'Completed' }))
                      }
                    />
                    Completed
                  </label>
                </div>
              </div>
            </div>
            <div className="field">
              <label>Description</label>
              <textarea
                className="input"
                style={{ minHeight: 60 }}
                value={ptoPlanForm.description}
                onChange={(e) =>
                  setPtoPlanForm((f) => ({
                    ...f,
                    description: e.target.value,
                  }))
                }
              />
            </div>
            <div
              className="mono"
              style={{ fontSize: 11, color: 'rgba(233,233,237,.42)' }}
            >
              Negative hours give time back (a half day worked during PTO).
            </div>
          </div>
        )}

        {store.modal === 'holiday' && (
          <div className="flex flex-col gap-[11px]">
            <div className="field">
              <label>Name</label>
              <input
                className="input mono"
                value={holidayForm.name}
                onChange={(e) =>
                  setHolidayForm((f) => ({ ...f, name: e.target.value }))
                }
                placeholder="Christmas Day"
              />
            </div>
            <div className="flex flex-col gap-[11px] sm:flex-row">
              <div className="field sm:flex-1">
                <label>Date</label>
                <input
                  className="input mono"
                  type="date"
                  value={holidayForm.date}
                  onChange={(e) =>
                    setHolidayForm((f) => ({ ...f, date: e.target.value }))
                  }
                />
              </div>
              <div className="field sm:w-[110px]">
                <label>Hours</label>
                <input
                  className="input mono"
                  type="number"
                  step="0.01"
                  value={holidayForm.hours}
                  onChange={(e) =>
                    setHolidayForm((f) => ({ ...f, hours: e.target.value }))
                  }
                  placeholder="8.00"
                />
              </div>
            </div>
          </div>
        )}

        <div className="dialog-actions">
          <button
            className="btn btn-secondary mono"
            onClick={store.closeModal}
            style={{ fontSize: 12.5 }}
          >
            Cancel
          </button>
          <button
            className="btn btn-primary mono"
            onClick={handleSave}
            style={{ fontSize: 12.5 }}
          >
            {store.modalMode}
          </button>
        </div>
      </div>
    </div>
  )
}
