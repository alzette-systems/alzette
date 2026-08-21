(function () {
  'use strict';

  var button = document.querySelector('[data-copy-invitation]');
  var input = document.querySelector('#invitation-link');
  var status = document.querySelector('#invitation-copy-status');
  if (!button || !input) return;

  function report(message) {
    if (status) status.textContent = message;
  }

  function selectLink() {
    input.focus();
    input.select();
    input.setSelectionRange(0, input.value.length);
  }

  button.addEventListener('click', function () {
    var value = input.value;
    if (!value) {
      report('The invitation link is unavailable. Resend the invitation.');
      return;
    }
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(value).then(function () {
        button.textContent = 'Copied';
        report('Invitation link copied.');
      }, function () {
        selectLink();
        report('Copy was blocked. The complete link is selected; press Command-C or Control-C.');
      });
      return;
    }
    selectLink();
    try {
      if (document.execCommand('copy')) {
        button.textContent = 'Copied';
        report('Invitation link copied.');
        return;
      }
    } catch (error) {
      // The selected-link fallback remains usable without clipboard access.
    }
    report('The complete link is selected; press Command-C or Control-C.');
  });
}());
