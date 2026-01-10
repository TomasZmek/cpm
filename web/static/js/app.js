/**
 * CPM - Caddy Proxy Manager
 * Main JavaScript file
 */

// Custom confirm handler - intercept before HTMX processes
document.addEventListener('click', function(evt) {
  const el = evt.target.closest('[hx-confirm]');
  if (!el) return;
  
  const question = el.getAttribute('hx-confirm');
  if (!question) return;
  
  // Prevent default click and HTMX from processing
  evt.preventDefault();
  evt.stopPropagation();
  
  Swal.fire({
    title: window.cpmLang === 'cs' ? 'Potvrzení' : 'Confirm',
    text: question,
    icon: 'warning',
    showCancelButton: true,
    confirmButtonColor: '#dc3545',
    cancelButtonColor: '#6c757d',
    confirmButtonText: window.cpmLang === 'cs' ? 'Ano, provést' : 'Yes, proceed',
    cancelButtonText: window.cpmLang === 'cs' ? 'Zrušit' : 'Cancel',
    reverseButtons: true
  }).then((result) => {
    if (result.isConfirmed) {
      // Remove hx-confirm temporarily to prevent double dialog
      el.removeAttribute('hx-confirm');
      // Trigger HTMX request
      htmx.trigger(el, 'click');
      // Restore hx-confirm
      setTimeout(() => el.setAttribute('hx-confirm', question), 100);
    }
  });
}, true); // Use capture phase to intercept before HTMX

// Initialize HTMX extensions
document.addEventListener('DOMContentLoaded', function() {
  document.body.addEventListener('htmx:beforeRequest', function(evt) {
    evt.target.classList.add('htmx-loading');
  });

  document.body.addEventListener('htmx:afterRequest', function(evt) {
    evt.target.classList.remove('htmx-loading');
  });

  // Handle flash messages
  const flashContainer = document.getElementById('flash-messages');
  if (flashContainer) {
    setTimeout(() => {
      flashContainer.querySelectorAll('.alert').forEach(alert => {
        alert.style.opacity = '0';
        setTimeout(() => alert.remove(), 300);
      });
    }, 5000);
  }

  // Initialize search with debounce
  const searchInput = document.getElementById('search-input');
  if (searchInput) {
    let timeout;
    searchInput.addEventListener('input', function() {
      clearTimeout(timeout);
      timeout = setTimeout(() => {
        htmx.trigger(searchInput, 'search');
      }, 300);
    });
  }

  // Confirm delete dialogs
  document.body.addEventListener('click', function(evt) {
    if (evt.target.matches('[data-confirm]')) {
      const message = evt.target.dataset.confirm || 'Are you sure?';
      if (!confirm(message)) {
        evt.preventDefault();
        evt.stopPropagation();
      }
    }
  });

  // Tab navigation
  document.querySelectorAll('.tabs .tab').forEach(tab => {
    tab.addEventListener('click', function() {
      const tabGroup = this.closest('.tabs');
      tabGroup.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      this.classList.add('active');
    });
  });

  // Toggle password visibility
  document.querySelectorAll('[data-toggle-password]').forEach(btn => {
    btn.addEventListener('click', function() {
      const input = document.getElementById(this.dataset.togglePassword);
      if (input) {
        input.type = input.type === 'password' ? 'text' : 'password';
        this.textContent = input.type === 'password' ? '👁️' : '🙈';
      }
    });
  });

  // Form validation
  document.querySelectorAll('form[data-validate]').forEach(form => {
    form.addEventListener('submit', function(evt) {
      let valid = true;
      
      form.querySelectorAll('[required]').forEach(input => {
        if (!input.value.trim()) {
          valid = false;
          input.classList.add('error');
        } else {
          input.classList.remove('error');
        }
      });

      if (!valid) {
        evt.preventDefault();
        alert('Please fill in all required fields');
      }
    });
  });

  // Copy to clipboard
  document.body.addEventListener('click', function(evt) {
    if (evt.target.matches('[data-copy]')) {
      const text = evt.target.dataset.copy;
      navigator.clipboard.writeText(text).then(() => {
        const originalText = evt.target.textContent;
        evt.target.textContent = '✓ Copied!';
        setTimeout(() => {
          evt.target.textContent = originalText;
        }, 2000);
      });
    }
  });
});

// Modal functions
function openModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.remove('hidden');
    document.body.style.overflow = 'hidden';
  }
}

function closeModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.add('hidden');
    document.body.style.overflow = '';
  }
}

// Close modal on overlay click
document.addEventListener('click', function(evt) {
  if (evt.target.classList.contains('modal-overlay')) {
    evt.target.classList.add('hidden');
    document.body.style.overflow = '';
  }
});

// Close modal on Escape key
document.addEventListener('keydown', function(evt) {
  if (evt.key === 'Escape') {
    document.querySelectorAll('.modal-overlay:not(.hidden)').forEach(modal => {
      modal.classList.add('hidden');
    });
    document.body.style.overflow = '';
  }
});

// Toast notifications
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container') || createToastContainer();
  
  const toast = document.createElement('div');
  toast.className = `alert alert-${type}`;
  toast.style.cssText = 'opacity: 0; transform: translateY(-10px); transition: all 0.3s;';
  toast.textContent = message;
  
  container.appendChild(toast);
  
  // Animate in
  requestAnimationFrame(() => {
    toast.style.opacity = '1';
    toast.style.transform = 'translateY(0)';
  });
  
  // Remove after 5 seconds
  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(-10px)';
    setTimeout(() => toast.remove(), 300);
  }, 5000);
}

function createToastContainer() {
  const container = document.createElement('div');
  container.id = 'toast-container';
  container.style.cssText = 'position: fixed; top: 1rem; right: 1rem; z-index: 1000; display: flex; flex-direction: column; gap: 0.5rem;';
  document.body.appendChild(container);
  return container;
}

// Alpine.js data helpers
document.addEventListener('alpine:init', () => {
  // Add any Alpine.js stores or magic helpers here
});
