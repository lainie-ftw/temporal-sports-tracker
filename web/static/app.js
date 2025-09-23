// Global state
let currentSport = '';
let currentLeague = '';
let refreshInterval = null;

// DOM elements
const sportSelect = document.getElementById('sport-select');
const leagueSelect = document.getElementById('league-select');
const teamsSelect = document.getElementById('teams-select');
const conferencesSelect = document.getElementById('conferences-select');
const teamsGroup = document.getElementById('teams-group');
const conferencesGroup = document.getElementById('conferences-group');
const trackingForm = document.getElementById('tracking-form');
const startTrackingBtn = document.getElementById('start-tracking-btn');
const refreshWorkflowsBtn = document.getElementById('refresh-workflows-btn');
const workflowsList = document.getElementById('workflows-list');
const workflowsCount = document.getElementById('workflows-count');
const statusMessage = document.getElementById('status-message');

// Initialize the app
document.addEventListener('DOMContentLoaded', function() {
    loadSports();
    loadWorkflows();
    setupEventListeners();
    
    // Auto-refresh workflows every 30 seconds
    refreshInterval = setInterval(loadWorkflows, 30000);
});

// Event listeners
function setupEventListeners() {
    // Sport selection
    sportSelect.addEventListener('change', function() {
        currentSport = this.value;
        if (currentSport) {
            loadLeagues(currentSport);
            leagueSelect.disabled = false;
        } else {
            resetLeagueAndBelow();
        }
    });

    // League selection
    leagueSelect.addEventListener('change', function() {
        currentLeague = this.value;
        if (currentLeague) {
            loadTeamsAndConferences();
            enableTrackingOptions();
        } else {
            resetTrackingOptions();
        }
    });

    // Track type radio buttons
    document.querySelectorAll('input[name="track-type"]').forEach(radio => {
        radio.addEventListener('change', function() {
            if (this.value === 'teams') {
                teamsGroup.style.display = 'block';
                conferencesGroup.style.display = 'none';
            } else {
                teamsGroup.style.display = 'none';
                conferencesGroup.style.display = 'block';
            }
            updateStartButtonState();
        });
    });

    // Selection changes
    teamsSelect.addEventListener('change', updateStartButtonState);
    conferencesSelect.addEventListener('change', updateStartButtonState);

    // Form submission
    trackingForm.addEventListener('submit', handleTrackingSubmit);

    // Refresh workflows
    refreshWorkflowsBtn.addEventListener('click', loadWorkflows);
}

// API calls
async function apiCall(url, options = {}) {
    try {
        const response = await fetch(url, {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        return await response.json();
    } catch (error) {
        console.error('API call failed:', error);
        showStatus('API call failed: ' + error.message, 'error');
        throw error;
    }
}

// Load sports
async function loadSports() {
    try {
        const sports = await apiCall('/api/sports');
        populateSelect(sportSelect, sports, 'id', 'name', 'Select a sport...');
    } catch (error) {
        showStatus('Failed to load sports', 'error');
    }
}

// Load leagues for selected sport
async function loadLeagues(sport) {
    try {
        resetLeagueAndBelow();
        const leagues = await apiCall(`/api/leagues/${sport}`);
        populateSelect(leagueSelect, leagues, 'id', 'name', 'Select a league...');
    } catch (error) {
        showStatus('Failed to load leagues', 'error');
    }
}

// Load teams and conferences
async function loadTeamsAndConferences() {
    try {
        // Load teams
        teamsSelect.innerHTML = '<option value="">Loading teams...</option>';
        teamsSelect.disabled = true;
        
        const teams = await apiCall(`/api/teams/${currentSport}/${currentLeague}`);
        populateSelect(teamsSelect, teams, 'id', 'displayName', '', true);
        teamsSelect.disabled = false;

        // Load conferences (for college sports)
        if (currentLeague.includes('college')) {
            conferencesSelect.innerHTML = '<option value="">Loading conferences...</option>';
            conferencesSelect.disabled = true;
            
            const conferences = await apiCall(`/api/conferences/${currentSport}/${currentLeague}`);
            populateSelect(conferencesSelect, conferences, 'id', 'name', '', true);
            conferencesSelect.disabled = false;
        } else {
            conferencesSelect.innerHTML = '<option value="">Not available for this league</option>';
            conferencesSelect.disabled = true;
        }
    } catch (error) {
        showStatus('Failed to load teams/conferences', 'error');
        teamsSelect.innerHTML = '<option value="">Failed to load teams</option>';
        conferencesSelect.innerHTML = '<option value="">Failed to load conferences</option>';
    }
}

// Load workflows
async function loadWorkflows() {
    try {
        const workflows = await apiCall('/api/workflows');
        displayWorkflows(workflows);
    } catch (error) {
        console.error('Failed to load workflows:', error);
        // Don't show error message for workflow loading failures to avoid spam
    }
}

// Handle form submission
async function handleTrackingSubmit(e) {
    e.preventDefault();
    
    const trackType = document.querySelector('input[name="track-type"]:checked').value;
    const selectedTeams = Array.from(teamsSelect.selectedOptions).map(option => option.value);
    const selectedConferences = Array.from(conferencesSelect.selectedOptions).map(option => option.value);
    
    const requestData = {
        sport: currentSport,
        league: currentLeague,
        teams: trackType === 'teams' ? selectedTeams : [],
        conferences: trackType === 'conferences' ? selectedConferences : []
    };

    try {
        startTrackingBtn.disabled = true;
        startTrackingBtn.textContent = 'Starting...';
        
        const response = await apiCall('/api/track', {
            method: 'POST',
            body: JSON.stringify(requestData)
        });
        
        showStatus('Tracking started successfully!', 'success');
        
        // Reset form
        trackingForm.reset();
        resetLeagueAndBelow();
        
        // Refresh workflows after a short delay
        setTimeout(loadWorkflows, 2000);
        
    } catch (error) {
        showStatus('Failed to start tracking', 'error');
    } finally {
        startTrackingBtn.disabled = false;
        startTrackingBtn.textContent = 'Start Tracking';
    }
}

// View workflow in Temporal UI
function viewWorkflow(workflowId, runId) {
    const temporalUrl = `https://cloud.temporal.io/namespaces/laine.sdvdw/workflows/${workflowId}/${runId}/history`;
    window.open(temporalUrl, '_blank');
}

function viewGame(apiRoot, gameId) {
    const gameUrl = ` https://www.espn.com/college-football/game/_/gameId/${gameId}`;
    window.open(gameUrl, '_blank');
}

// Helper functions
function populateSelect(selectElement, items, valueField, textField, placeholder = '', multiple = false) {
    selectElement.innerHTML = '';
    
    if (!multiple && placeholder) {
        const option = document.createElement('option');
        option.value = '';
        option.textContent = placeholder;
        selectElement.appendChild(option);
    }
    
    items.forEach(item => {
        const option = document.createElement('option');
        option.value = item[valueField];
        option.textContent = item[textField];
        selectElement.appendChild(option);
    });
}

function resetLeagueAndBelow() {
    leagueSelect.innerHTML = '<option value="">Select a league...</option>';
    leagueSelect.disabled = true;
    currentLeague = '';
    resetTrackingOptions();
}

function resetTrackingOptions() {
    teamsSelect.innerHTML = '<option value="">Select a league first</option>';
    teamsSelect.disabled = true;
    conferencesSelect.innerHTML = '<option value="">Select a league first</option>';
    conferencesSelect.disabled = true;
    startTrackingBtn.disabled = true;
}

function enableTrackingOptions() {
    teamsSelect.innerHTML = '<option value="">Loading teams...</option>';
    conferencesSelect.innerHTML = '<option value="">Loading conferences...</option>';
}

function updateStartButtonState() {
    const trackType = document.querySelector('input[name="track-type"]:checked').value;
    let hasSelection = false;
    
    if (trackType === 'teams') {
        hasSelection = teamsSelect.selectedOptions.length > 0;
    } else {
        hasSelection = conferencesSelect.selectedOptions.length > 0;
    }
    
    startTrackingBtn.disabled = !hasSelection || !currentSport || !currentLeague;
}

function displayWorkflows(workflows) {
    workflowsCount.textContent = `${workflows.length} active workflow${workflows.length !== 1 ? 's' : ''}`;
    
    if (workflows.length === 0) {
        workflowsList.innerHTML = `
            <div class="no-workflows">
                <p>No active game workflows found.</p>
                <p>Start tracking some teams to see workflows here!</p>
            </div>
        `;
        return;
    }
    
    const workflowsHTML = workflows.map(workflow => `
        <div class="workflow-item">
            <div class="workflow-header">
                <div class="workflow-title">
                    ${workflow.homeTeam && workflow.awayTeam ? 
                        `${workflow.homeTeam} vs ${workflow.awayTeam}` : ''}
                    ${workflow.startTime ? 
                    `<div>${new Date(workflow.startTime).toLocaleString()}</div>` : ''}
                </div>
                <div class="workflow-status ${workflow.status.toLowerCase()}">
                    ${workflow.status}
                </div>
            </div>
            <div class="workflow-details">
                <div><strong>Workflow ID:</strong> ${workflow.workflowId}</div>
                <div><strong>Run ID:</strong> ${workflow.runId}</div>
            </div>
            <div class="workflow-actions">
                <button class="temporal-btn" onclick="viewWorkflow('${workflow.workflowId}', '${workflow.runId}')" alt="View Workflow in Temporal UI">
                &nbsp;&nbsp;&nbsp;
                </button>
                <button onclick="viewGame('${workflow.gameId}', '${workflow.gameId}')" alt="View Game Info on ESPN">
                View Game Info at ESPN.com
                </button>
                                
            </div>
        </div>
    `).join('');
    
    workflowsList.innerHTML = workflowsHTML;
}

function showStatus(message, type = 'info') {
    statusMessage.textContent = message;
    statusMessage.className = `status-message ${type} show`;
    
    // Auto-hide after 5 seconds
    setTimeout(() => {
        statusMessage.classList.remove('show');
    }, 5000);
}

// Cleanup on page unload
window.addEventListener('beforeunload', function() {
    if (refreshInterval) {
        clearInterval(refreshInterval);
    }
});
