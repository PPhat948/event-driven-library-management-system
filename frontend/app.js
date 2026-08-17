/**
 * Library Management System — Frontend Logic (Vanilla JS)
 * Handles API calls to Book, Inventory, and Notification microservices.
 */

// ─── API Configuration & Auto-Detection ──────────────────────────────────────────
const API = {
  books: '/api/books',
  inventory: '/api/inventory',
  notifications: '/api/notifications',
  health: {
    book: '/api/books/health',
    inventory: '/api/inventory/health',
    notification: '/api/notifications/health'
  }
};

// Fallback to direct ports if opened directly in browser without Nginx reverse proxy
if (window.location.port !== '80' && window.location.port !== '' && window.location.protocol.startsWith('http')) {
  if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
    // If not proxied by nginx, direct port access can be configured
    API.books = 'http://localhost:8001/books';
    API.inventory = 'http://localhost:8002/inventory';
    API.notifications = 'http://localhost:8003/notifications';
    API.health = {
      book: 'http://localhost:8001/health',
      inventory: 'http://localhost:8002/health',
      notification: 'http://localhost:8003/health'
    };
  }
}

// ─── Application State ──────────────────────────────────────────────────────────
const state = {
  books: [],
  inventoryMap: new Map(), // book_id -> inventory item
  inventoryList: [],
  notifications: [],
  activeTab: 'catalog',
  searchQuery: '',
  statusFilter: 'all',
  isAutoRefresh: true,
  refreshTimer: null,
  isFetching: false
};

// ─── DOM Element References ────────────────────────────────────────────────────
const elements = {
  // Navigation & Status
  tabCatalog: document.getElementById('tab-catalog'),
  tabInventory: document.getElementById('tab-inventory'),
  tabActivity: document.getElementById('tab-activity'),
  activityBadge: document.getElementById('activity-badge'),
  viewCatalog: document.getElementById('view-catalog'),
  viewInventory: document.getElementById('view-inventory'),
  viewActivity: document.getElementById('view-activity'),
  systemStatus: document.getElementById('system-status'),
  systemStatusText: document.getElementById('system-status-text'),

  // Quick Stats
  statTotalTitles: document.getElementById('stat-total-titles'),
  statAvailableCopies: document.getElementById('stat-available-copies'),
  statBorrowedCopies: document.getElementById('stat-borrowed-copies'),
  statStockAlerts: document.getElementById('stat-stock-alerts'),

  // Controls
  searchInput: document.getElementById('search-input'),
  statusFilter: document.getElementById('status-filter'),
  autoRefreshToggle: document.getElementById('auto-refresh-toggle'),
  refreshBtn: document.getElementById('refresh-btn'),
  openAddBookBtn: document.getElementById('open-add-book-btn'),

  // Tables & Feeds
  catalogTableBody: document.getElementById('catalog-table-body'),
  inventoryContainer: document.getElementById('inventory-container'),
  activityTableBody: document.getElementById('activity-table-body'),

  // Modals & Forms
  modalAddBook: document.getElementById('modal-add-book'),
  formAddBook: document.getElementById('form-add-book'),

  modalBorrowBook: document.getElementById('modal-borrow-book'),
  formBorrowBook: document.getElementById('form-borrow-book'),
  borrowBookId: document.getElementById('borrow-book-id'),
  borrowBookTitle: document.getElementById('borrow-book-title'),
  borrowBookMeta: document.getElementById('borrow-book-meta'),
  borrowDueDate: document.getElementById('borrow-due-date'),

  modalReturnBook: document.getElementById('modal-return-book'),
  formReturnBook: document.getElementById('form-return-book'),
  returnBookId: document.getElementById('return-book-id'),
  returnBookTitle: document.getElementById('return-book-title'),
  returnBookMeta: document.getElementById('return-book-meta'),
  returnBorrowerSelect: document.getElementById('return-borrower-select'),

  // Event Inspector Modal
  modalEventDetail: document.getElementById('modal-event-detail'),
  eventDetailBadge: document.getElementById('event-detail-badge'),
  eventDetailCorrelation: document.getElementById('event-detail-correlation'),
  eventDetailId: document.getElementById('event-detail-id'),
  eventDetailTimestamp: document.getElementById('event-detail-timestamp'),
  eventDetailJson: document.getElementById('event-detail-json'),
  btnCopyCorrelation: document.getElementById('btn-copy-correlation'),

  toastContainer: document.getElementById('toast-container')
};

// ─── Initialization ────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  setupEventListeners();
  setDefaultDueDate();
  loadAllData();
  startAutoRefresh();
});

function setDefaultDueDate() {
  // Set default due date to 14 days from now
  const nextTwoWeeks = new Date();
  nextTwoWeeks.setDate(nextTwoWeeks.getDate() + 14);
  elements.borrowDueDate.value = nextTwoWeeks.toISOString().split('T')[0];
}

// ─── Event Listeners ───────────────────────────────────────────────────────────
function setupEventListeners() {
  // Tab Navigation
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const tab = btn.getAttribute('data-tab');
      switchTab(tab);
    });
  });

  // Search & Filter
  elements.searchInput.addEventListener('input', (e) => {
    state.searchQuery = e.target.value.trim().toLowerCase();
    handleFilterChange();
  });

  elements.statusFilter.addEventListener('change', (e) => {
    state.statusFilter = e.target.value;
    handleFilterChange();
  });

  // Controls
  elements.refreshBtn.addEventListener('click', () => loadAllData(true));

  elements.autoRefreshToggle.addEventListener('change', (e) => {
    state.isAutoRefresh = e.target.checked;
    if (state.isAutoRefresh) {
      startAutoRefresh();
      showToast('Auto-refresh enabled (2s)');
    } else {
      stopAutoRefresh();
      showToast('Auto-refresh paused');
    }
  });

  // Modal Open / Close
  elements.openAddBookBtn.addEventListener('click', () => openModal(elements.modalAddBook));

  document.querySelectorAll('[data-close-modal]').forEach(el => {
    el.addEventListener('click', () => {
      closeAllModals();
    });
  });

  document.querySelectorAll('.modal-overlay').forEach(overlay => {
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) {
        closeAllModals();
      }
    });
  });

  // Copy Correlation ID button
  if (elements.btnCopyCorrelation) {
    elements.btnCopyCorrelation.addEventListener('click', () => {
      const corr = elements.eventDetailCorrelation.textContent;
      if (corr && corr !== '-') {
        navigator.clipboard.writeText(corr);
        showToast('Correlation ID copied to clipboard');
      }
    });
  }

  // Form Submissions
  elements.formAddBook.addEventListener('submit', handleAddBookSubmit);
  elements.formBorrowBook.addEventListener('submit', handleBorrowBookSubmit);
  elements.formReturnBook.addEventListener('submit', handleReturnBookSubmit);
}

// ─── Tab Switching ─────────────────────────────────────────────────────────────
function switchTab(tabName) {
  state.activeTab = tabName;

  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.classList.toggle('active', btn.getAttribute('data-tab') === tabName);
  });

  elements.viewCatalog.classList.toggle('active', tabName === 'catalog');
  elements.viewInventory.classList.toggle('active', tabName === 'inventory');
  elements.viewActivity.classList.toggle('active', tabName === 'activity');

  // Update search placeholder according to active tab context
  if (tabName === 'catalog') {
    elements.searchInput.placeholder = 'Search by title, author, or ISBN...';
    elements.statusFilter.style.display = 'inline-block';
  } else if (tabName === 'inventory') {
    elements.searchInput.placeholder = 'Search inventory by book title...';
    elements.statusFilter.style.display = 'inline-block';
  } else if (tabName === 'activity') {
    elements.searchInput.placeholder = 'Search events by type, correlation ID, or message...';
    elements.statusFilter.style.display = 'none';
    elements.activityBadge.style.display = 'none';
  }

  handleFilterChange();
}

function handleFilterChange() {
  if (state.activeTab === 'catalog') {
    renderCatalogTable();
  } else if (state.activeTab === 'inventory') {
    renderInventoryTable();
  } else if (state.activeTab === 'activity') {
    renderActivityList();
  }
}

// ─── Polling & Data Fetching ───────────────────────────────────────────────────
function startAutoRefresh() {
  stopAutoRefresh();
  state.refreshTimer = setInterval(() => {
    if (state.isAutoRefresh && !state.isFetching) {
      loadAllData(false);
    }
  }, 2000);
}

function stopAutoRefresh() {
  if (state.refreshTimer) {
    clearInterval(state.refreshTimer);
    state.refreshTimer = null;
  }
}

async function loadAllData(showFeedback = false) {
  state.isFetching = true;
  if (showFeedback) {
    elements.refreshBtn.textContent = 'Updating...';
  }

  try {
    const [booksRes, inventoryRes, notifsRes, healthRes] = await Promise.allSettled([
      fetchJSON(API.books),
      fetchJSON(API.inventory),
      fetchJSON(`${API.notifications}?limit=50`),
      checkHealth()
    ]);

    // Handle Books response
    if (booksRes.status === 'fulfilled' && booksRes.value) {
      state.books = booksRes.value.books || [];
    }

    // Handle Inventory response
    if (inventoryRes.status === 'fulfilled' && inventoryRes.value) {
      state.inventoryList = inventoryRes.value.inventory || [];
      state.inventoryMap.clear();
      state.inventoryList.forEach(item => {
        state.inventoryMap.set(item.book_id, item);
      });
    }

    // Handle Notifications response
    if (notifsRes.status === 'fulfilled' && notifsRes.value) {
      state.notifications = notifsRes.value.notifications || [];
    }

    // Handle System Health status
    if (healthRes.status === 'fulfilled') {
      renderSystemHealth(healthRes.value);
    }

    // Render all UI components
    renderStats();
    renderCatalogTable();
    renderInventoryTable();
    renderActivityList();

    if (showFeedback) {
      showToast('Data refreshed');
    }
  } catch (err) {
    console.error('Error fetching data:', err);
    if (showFeedback) {
      showToast('Failed to refresh data', 'error');
    }
  } finally {
    state.isFetching = false;
    if (showFeedback) {
      elements.refreshBtn.textContent = 'Refresh';
    }
  }
}

async function checkHealth() {
  try {
    const [hBook, hInv, hNotif] = await Promise.all([
      fetchJSON(API.health.book).catch(() => null),
      fetchJSON(API.health.inventory).catch(() => null),
      fetchJSON(API.health.notification).catch(() => null)
    ]);

    const bookOk = hBook?.status === 'ok';
    const invOk = hInv?.status === 'ok';
    const notifOk = hNotif?.status === 'ok';

    return {
      allOk: bookOk && invOk && notifOk,
      bookOk,
      invOk,
      notifOk
    };
  } catch {
    return { allOk: false };
  }
}

// ─── Render Functions ──────────────────────────────────────────────────────────

function renderSystemHealth(health) {
  if (health.allOk) {
    elements.systemStatus.className = 'status-indicator online';
    elements.systemStatusText.textContent = 'All services online';
  } else {
    elements.systemStatus.className = 'status-indicator offline';
    const degraded = [];
    if (!health.bookOk) degraded.push('Book');
    if (!health.invOk) degraded.push('Inventory');
    if (!health.notifOk) degraded.push('Notification');
    elements.systemStatusText.textContent = `Degraded (${degraded.join(', ')})`;
  }
}

function renderStats() {
  const totalTitles = state.books.length;
  let totalAvailable = 0;
  let totalBorrowed = 0;
  let stockAlerts = 0;

  state.books.forEach(b => {
    const inv = state.inventoryMap.get(b.id);
    const available = inv ? inv.available_count : b.available_count;
    const borrowed = inv ? inv.borrowed_count : (b.total_quantity - b.available_count);

    totalAvailable += available;
    totalBorrowed += borrowed;

    if (available === 0 || (inv && inv.status === 'out_of_stock')) {
      stockAlerts++;
    } else if (available <= 2 || (inv && inv.status === 'low_stock')) {
      stockAlerts++;
    }
  });

  elements.statTotalTitles.textContent = totalTitles;
  elements.statAvailableCopies.textContent = totalAvailable;
  elements.statBorrowedCopies.textContent = totalBorrowed;
  elements.statStockAlerts.textContent = stockAlerts;
}

function renderCatalogTable() {
  if (state.books.length === 0) {
    elements.catalogTableBody.innerHTML = `
      <tr>
        <td colspan="5" class="empty-state">No books in catalog. Click "Add Book" to get started.</td>
      </tr>
    `;
    return;
  }

  // Filter books by search & status
  const filtered = state.books.filter(book => {
    const inv = state.inventoryMap.get(book.id);
    const available = inv ? inv.available_count : book.available_count;
    const status = inv ? inv.status : (available === 0 ? 'out_of_stock' : (available <= 2 ? 'low_stock' : 'available'));

    // Status filter
    if (state.statusFilter !== 'all' && status !== state.statusFilter) {
      return false;
    }

    // Search query
    if (state.searchQuery) {
      const matchTitle = book.title.toLowerCase().includes(state.searchQuery);
      const matchAuthor = book.author.toLowerCase().includes(state.searchQuery);
      const matchISBN = book.isbn.toLowerCase().includes(state.searchQuery);
      return matchTitle || matchAuthor || matchISBN;
    }

    return true;
  });

  if (filtered.length === 0) {
    elements.catalogTableBody.innerHTML = `
      <tr>
        <td colspan="5" class="empty-state">No books matching filter criteria.</td>
      </tr>
    `;
    return;
  }

  elements.catalogTableBody.innerHTML = filtered.map(book => {
    const inv = state.inventoryMap.get(book.id);
    const available = inv ? inv.available_count : book.available_count;
    const total = inv ? inv.total_quantity : book.total_quantity;

    // Status computation
    let badgeClass = 'badge-available';
    let statusLabel = 'Available';

    if (available === 0) {
      badgeClass = 'badge-out';
      statusLabel = 'Out of stock';
    } else if (available <= 2) {
      badgeClass = 'badge-low';
      statusLabel = 'Low stock';
    }

    return `
      <tr>
        <td>
          <div class="book-title-cell">
            <span class="book-title-text">${escapeHTML(book.title)}</span>
            <span class="book-author-text">${escapeHTML(book.author)}</span>
          </div>
        </td>
        <td>
          <span class="isbn-code">${escapeHTML(book.isbn)}</span>
        </td>
        <td>
          <span class="badge ${badgeClass}">${statusLabel}</span>
        </td>
        <td>
          <strong>${available}</strong> / ${total} copies
        </td>
        <td class="text-right">
          <div class="action-group">
            <button 
              class="btn btn-secondary btn-sm" 
              onclick="openBorrowModal('${book.id}', '${escapeAttr(book.title)}', ${available}, ${total})"
              ${available === 0 ? 'disabled title="Out of stock"' : ''}
            >
              Borrow
            </button>
            <button 
              class="btn btn-secondary btn-sm" 
              onclick="openReturnModal('${book.id}', '${escapeAttr(book.title)}', ${available}, ${total})"
              ${available >= total ? 'disabled title="All copies are in library"' : ''}
            >
              Return
            </button>
            <button 
              class="btn btn-danger-outline btn-sm" 
              onclick="deleteBook('${book.id}', '${escapeAttr(book.title)}')"
              title="Delete book"
            >
              Delete
            </button>
          </div>
        </td>
      </tr>
    `;
  }).join('');
}

function renderInventoryTable() {
  if (!elements.inventoryContainer) return;

  if (state.inventoryList.length === 0) {
    elements.inventoryContainer.innerHTML = `
      <div class="empty-state">No inventory data available yet.</div>
    `;
    return;
  }

  const filtered = state.inventoryList.filter(item => {
    const status = (item.status === 'out_of_stock' || item.available_count === 0)
      ? 'out_of_stock'
      : (item.status === 'low_stock' || item.available_count <= 2 ? 'low_stock' : 'available');

    // Status filter
    if (state.statusFilter !== 'all' && status !== state.statusFilter) {
      return false;
    }

    // Search query
    if (state.searchQuery) {
      const matchTitle = (item.title || '').toLowerCase().includes(state.searchQuery);
      const matchAuthor = (item.author || '').toLowerCase().includes(state.searchQuery);
      return matchTitle || matchAuthor;
    }

    return true;
  });

  if (filtered.length === 0) {
    elements.inventoryContainer.innerHTML = `
      <div class="empty-state">No inventory items matching filter criteria.</div>
    `;
    return;
  }

  // Define status groups
  const groups = [
    { key: 'out_of_stock', title: 'Out of Stock', badgeClass: 'badge-out', fillClass: 'fill-red', items: [] },
    { key: 'low_stock', title: 'Low Stock (≤ 2)', badgeClass: 'badge-low', fillClass: 'fill-amber', items: [] },
    { key: 'available', title: 'Available', badgeClass: 'badge-available', fillClass: 'fill-green', items: [] }
  ];

  filtered.forEach(item => {
    const status = (item.status === 'out_of_stock' || item.available_count === 0)
      ? 'out_of_stock'
      : (item.status === 'low_stock' || item.available_count <= 2 ? 'low_stock' : 'available');
    const grp = groups.find(g => g.key === status) || groups[2];
    grp.items.push(item);
  });

  elements.inventoryContainer.innerHTML = groups
    .filter(g => g.items.length > 0)
    .map(g => {
      const countLabel = `${g.items.length} ${g.items.length === 1 ? 'title' : 'titles'}`;

      const rowsHTML = g.items.map(item => {
        const total = item.total_quantity || 0;
        const available = item.available_count || 0;
        const borrowed = item.borrowed_count || 0;
        const pct = total > 0 ? Math.min(100, Math.round((borrowed / total) * 100)) : 0;
        const updatedTime = item.updated_at ? new Date(item.updated_at).toLocaleTimeString() : '-';

        return `
          <tr>
            <td>
              <div class="book-title-cell">
                <span class="book-title-text">${escapeHTML(item.title)}</span>
                <span class="book-author-text">${escapeHTML(item.author || '-')}</span>
              </div>
            </td>
            <td>${total}</td>
            <td><strong>${available}</strong></td>
            <td>${borrowed}</td>
            <td>
              <div class="stock-bar-cell">
                <div class="stock-bar-track">
                  <div class="stock-bar-fill ${g.fillClass}" style="width: ${pct}%;"></div>
                </div>
                <span class="stock-bar-text">${pct}%</span>
              </div>
            </td>
            <td><span class="activity-time">${updatedTime}</span></td>
          </tr>
        `;
      }).join('');

      return `
        <div class="inventory-group">
          <div class="inventory-group-header">
            <div class="inventory-group-title">
              <span class="badge ${g.badgeClass}">${g.title}</span>
            </div>
            <span class="inventory-group-count">${countLabel}</span>
          </div>
          <div class="table-container">
            <table class="data-table">
              <thead>
                <tr>
                  <th>Title & Author</th>
                  <th>Total</th>
                  <th>Available</th>
                  <th>Borrowed</th>
                  <th>Stock Utilization</th>
                  <th>Last Synced</th>
                </tr>
              </thead>
              <tbody>
                ${rowsHTML}
              </tbody>
            </table>
          </div>
        </div>
      `;
    }).join('');
}

function renderActivityList() {
  if (!elements.activityTableBody) return;

  if (state.notifications.length === 0) {
    elements.activityTableBody.innerHTML = `
      <tr>
        <td colspan="5" class="empty-state">No events recorded in system yet.</td>
      </tr>
    `;
    return;
  }

  const filtered = state.notifications.filter(n => {
    if (!state.searchQuery) return true;
    const q = state.searchQuery;
    const matchType = (n.event_type || '').toLowerCase().includes(q);
    const matchMsg = (n.message || '').toLowerCase().includes(q);
    const matchCorr = (n.correlation_id || '').toLowerCase().includes(q);
    const matchBook = (n.book_title || '').toLowerCase().includes(q);
    const matchMember = (n.member_id || '').toLowerCase().includes(q);
    const matchId = (n.id || '').toLowerCase().includes(q);
    return matchType || matchMsg || matchCorr || matchBook || matchMember || matchId;
  });

  if (filtered.length === 0) {
    elements.activityTableBody.innerHTML = `
      <tr>
        <td colspan="5" class="empty-state">No events matching search query.</td>
      </tr>
    `;
    return;
  }

  elements.activityTableBody.innerHTML = filtered.map(n => {
    const timeStr = n.created_at ? new Date(n.created_at).toLocaleTimeString() : '';
    const eventType = n.event_type || 'event';
    const msg = n.message || '-';
    const corrId = n.correlation_id || '';
    const shortCorr = corrId ? `${corrId.substring(0, 16)}...` : `#${(n.id || '').substring(0, 8)}`;

    return `
      <tr>
        <td>
          <span class="activity-time">${timeStr}</span>
        </td>
        <td>
          <span class="badge badge-event">${escapeHTML(eventType)}</span>
        </td>
        <td>
          <span class="activity-msg">${escapeHTML(msg)}</span>
        </td>
        <td>
          <code class="trace-code" title="Correlation ID: ${escapeAttr(corrId || n.id)}">${escapeHTML(shortCorr)}</code>
        </td>
        <td class="text-right">
          <button class="btn btn-secondary btn-sm" onclick="openEventInspector('${n.id}')" title="Inspect Event Envelope & Trace">
            Inspect
          </button>
        </td>
      </tr>
    `;
  }).join('');
}

// ─── Modal Actions (Add, Borrow, Return, Delete) ───────────────────────────────

function openModal(modalEl) {
  modalEl.classList.add('active');
  modalEl.setAttribute('aria-hidden', 'false');
}

function closeAllModals() {
  document.querySelectorAll('.modal-overlay').forEach(modal => {
    modal.classList.remove('active');
    modal.setAttribute('aria-hidden', 'true');
  });
}

// Global modal triggers for inline onclicks
window.openBorrowModal = function (bookId, title, available, total) {
  elements.borrowBookId.value = bookId;
  elements.borrowBookTitle.textContent = title;
  elements.borrowBookMeta.textContent = `Available: ${available} of ${total} copies`;
  setDefaultDueDate();
  openModal(elements.modalBorrowBook);
};

window.openReturnModal = async function (bookId, title, available, total) {
  elements.returnBookId.value = bookId;
  elements.returnBookTitle.textContent = title;
  elements.returnBookMeta.textContent = `Current available: ${available} / ${total} copies`;

  elements.returnBorrowerSelect.innerHTML = '<option value="">Loading active borrowers...</option>';
  elements.returnBorrowerSelect.disabled = true;
  openModal(elements.modalReturnBook);

  try {
    const data = await fetchJSON(`${API.books}/${bookId}/borrows`);
    const borrows = data.borrows || [];

    if (borrows.length === 0) {
      elements.returnBorrowerSelect.innerHTML = '<option value="">No active borrows found</option>';
      elements.returnBorrowerSelect.disabled = true;
      return;
    }

    elements.returnBorrowerSelect.innerHTML = borrows.map(b => {
      const name = b.member_name ? `${escapeHTML(b.member_name)} (${escapeHTML(b.member_id)})` : escapeHTML(b.member_id);
      const time = b.borrowed_at ? new Date(b.borrowed_at).toLocaleDateString() : '';
      return `<option value="${escapeAttr(b.member_id)}">${name} — Borrowed: ${time}</option>`;
    }).join('');
    elements.returnBorrowerSelect.disabled = false;
  } catch (err) {
    elements.returnBorrowerSelect.innerHTML = '<option value="">Failed to load borrowers</option>';
    showToast('Failed to load active borrowers', 'error');
  }
};

window.openEventInspector = function (id) {
  const notif = state.notifications.find(n => n.id === id);
  if (!notif) return;

  elements.eventDetailBadge.textContent = notif.event_type || 'event';
  elements.eventDetailCorrelation.textContent = notif.correlation_id || notif.event_id || '-';
  elements.eventDetailId.textContent = notif.event_id || notif.id || '-';
  elements.eventDetailTimestamp.textContent = notif.created_at ? new Date(notif.created_at).toLocaleString() : '-';

  // Construct standard Event Envelope representation
  const envelope = {
    event_id: notif.event_id || notif.id,
    event_type: notif.event_type,
    source_service: notif.event_type.startsWith('inventory.') ? 'inventory-service' : 'book-service',
    schema_version: '1.0',
    timestamp: notif.created_at,
    correlation_id: notif.correlation_id || notif.event_id,
    payload: {
      book_id: notif.book_id || undefined,
      book_title: notif.book_title || undefined,
      member_id: notif.member_id || undefined,
      message: notif.message
    }
  };

  elements.eventDetailJson.textContent = JSON.stringify(envelope, null, 2);
  openModal(elements.modalEventDetail);
};

window.deleteBook = async function (bookId, title) {
  if (!confirm(`Are you sure you want to delete "${title}"?`)) {
    return;
  }

  const corrId = generateCorrelationID();

  try {
    const res = await fetch(`${API.books}/${bookId}`, {
      method: 'DELETE',
      headers: {
        'X-Correlation-ID': corrId
      }
    });

    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || 'Failed to delete book');
    }

    showToast(`Deleted "${title}" (trace: ${corrId.substring(0, 10)}...)`);
    loadAllData();
  } catch (err) {
    showToast(err.message, 'error');
  }
};

async function handleAddBookSubmit(e) {
  e.preventDefault();
  const title = document.getElementById('book-title').value.trim();
  const author = document.getElementById('book-author').value.trim();
  const isbn = document.getElementById('book-isbn').value.trim();
  const totalQuantity = parseInt(document.getElementById('book-quantity').value, 10);

  if (!title || !author || !isbn || !totalQuantity) {
    showToast('Please fill in all required fields', 'error');
    return;
  }

  const corrId = generateCorrelationID();
  const submitBtn = document.getElementById('btn-submit-add-book');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Saving...';

  try {
    const res = await fetch(API.books, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Correlation-ID': corrId
      },
      body: JSON.stringify({
        title,
        author,
        isbn,
        total_quantity: totalQuantity
      })
    });

    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || 'Failed to create book');
    }

    showToast(`Created "${title}" (trace: ${corrId.substring(0, 10)}...)`);
    elements.formAddBook.reset();
    closeAllModals();
    loadAllData();
  } catch (err) {
    showToast(err.message, 'error');
  } finally {
    submitBtn.disabled = false;
    submitBtn.textContent = 'Save Book';
  }
}

async function handleBorrowBookSubmit(e) {
  e.preventDefault();
  const bookId = elements.borrowBookId.value;
  const memberId = document.getElementById('borrow-member-id').value.trim();
  const memberName = document.getElementById('borrow-member-name').value.trim();
  const dueDateVal = elements.borrowDueDate.value;

  if (!bookId || !memberId || !dueDateVal) {
    showToast('Member ID and Due Date are required', 'error');
    return;
  }

  const corrId = generateCorrelationID();
  const dueDate = new Date(`${dueDateVal}T23:59:59Z`).toISOString();
  const submitBtn = document.getElementById('btn-submit-borrow');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Processing...';

  try {
    const res = await fetch(`${API.books}/${bookId}/borrow`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Correlation-ID': corrId
      },
      body: JSON.stringify({
        member_id: memberId,
        member_name: memberName,
        due_date: dueDate
      })
    });

    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || 'Failed to borrow book');
    }

    showToast(`Book borrowed by ${memberId} (trace: ${corrId.substring(0, 10)}...)`);
    closeAllModals();
    loadAllData();
  } catch (err) {
    showToast(err.message, 'error');
  } finally {
    submitBtn.disabled = false;
    submitBtn.textContent = 'Confirm Borrow';
  }
}

async function handleReturnBookSubmit(e) {
  e.preventDefault();
  const bookId = elements.returnBookId.value;
  const memberId = elements.returnBorrowerSelect.value;

  if (!bookId || !memberId) {
    showToast('Please select a borrower to return', 'error');
    return;
  }

  const corrId = generateCorrelationID();
  const submitBtn = document.getElementById('btn-submit-return');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Processing...';

  try {
    const res = await fetch(`${API.books}/${bookId}/return`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Correlation-ID': corrId
      },
      body: JSON.stringify({
        member_id: memberId
      })
    });

    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || 'Failed to return book');
    }

    showToast(`Book returned for ${memberId} (trace: ${corrId.substring(0, 10)}...)`);
    closeAllModals();
    loadAllData();
  } catch (err) {
    showToast(err.message, 'error');
  } finally {
    submitBtn.disabled = false;
    submitBtn.textContent = 'Confirm Return';
  }
}

function generateCorrelationID() {
  return 'corr-' + Math.random().toString(36).substring(2, 10) + '-' + Date.now().toString(36);
}

// ─── Utilities ─────────────────────────────────────────────────────────────────

async function fetchJSON(url) {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`HTTP error ${res.status} from ${url}`);
  }
  return res.json();
}

function showToast(message, type = 'info') {
  const toast = document.createElement('div');
  toast.className = `toast ${type === 'error' ? 'toast-error' : ''}`;
  toast.textContent = message;

  elements.toastContainer.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    toast.style.transition = 'all 0.2s ease';
    setTimeout(() => toast.remove(), 200);
  }, 3000);
}

function escapeHTML(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

function escapeAttr(str) {
  if (!str) return '';
  return String(str)
    .replace(/"/g, '&quot;')
    .replace(/'/g, "\\'");
}
