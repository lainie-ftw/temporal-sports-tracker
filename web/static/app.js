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
const scheduleTypeSelect = document.getElementById('schedule-type');
const trackingForm = document.getElementById('tracking-form');
const startTrackingBtn = document.getElementById('start-tracking-btn');
const refreshWorkflowsBtn = document.getElementById('refresh-workflows-btn');
const refreshSchedulesBtn = document.getElementById('refresh-schedules-btn');
const workflowsList = document.getElementById('workflows-list');
const workflowsCount = document.getElementById('workflows-count');
const schedulesList = document.getElementById('schedules-list');
const schedulesCount = document.getElementById('schedules-count');
const statusMessage = document.getElementById('status-message');

// Initialize the app
document.addEventListener('DOMContentLoaded', function() {
    loadSports();
    loadWorkflows();
    loadSchedules();
    setupEventListeners();
    
    // Auto-refresh workflows and schedules every 30 seconds
    refreshInterval = setInterval(() => {
        loadWorkflows();
        loadSchedules();
    }, 30000);
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

    // Refresh workflows and schedules
    refreshWorkflowsBtn.addEventListener('click', loadWorkflows);
    refreshSchedulesBtn.addEventListener('click', loadSchedules);
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
    const scheduleType = scheduleTypeSelect.value;
    
    const requestData = {
        sport: currentSport,
        league: currentLeague,
        teams: trackType === 'teams' ? selectedTeams : [],
        conferences: trackType === 'conferences' ? selectedConferences : [],
        scheduleType: scheduleType
    };

    try {
        startTrackingBtn.disabled = true;
        startTrackingBtn.textContent = 'Starting...';
        
        const response = await apiCall('/api/track', {
            method: 'POST',
            body: JSON.stringify(requestData)
        });
        
        const message = scheduleType === 'once' ? 'Tracking started successfully!' : 
                       `Schedule created successfully (${scheduleType})!`;
        showStatus(message, 'success');
        
        // Reset form
        trackingForm.reset();
        resetLeagueAndBelow();
        
        // Refresh workflows and schedules after a short delay
        setTimeout(() => {
            loadWorkflows();
            loadSchedules();
        }, 2000);
        
    } catch (error) {
        showStatus('Failed to start tracking', 'error');
    } finally {
        startTrackingBtn.disabled = false;
        startTrackingBtn.textContent = 'Start Tracking';
    }
}

// View workflow in Temporal UI
function viewWorkflow(workflowUrl) {
    const temporalUrl = `${workflowUrl}/history`;
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
                        `${workflow.homeTeam} (${workflow.homeScore}) vs ${workflow.awayTeam} (${workflow.awayScore})` : ''}
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
                <button class="temporal-btn" onclick="viewWorkflow('${workflow.workflowUrl}')" alt="View Workflow in Temporal UI">
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

// Load schedules
async function loadSchedules() {
    try {
        const schedules = await apiCall('/api/schedules');
        displaySchedules(schedules);
    } catch (error) {
        console.error('Failed to load schedules:', error);
        // Don't show error message for schedule loading failures to avoid spam
    }
}

// Display schedules
function displaySchedules(schedules) {
    schedulesCount.textContent = `${schedules.length} active schedule${schedules.length !== 1 ? 's' : ''}`;
    
    if (schedules.length === 0) {
        schedulesList.innerHTML = `
            <div class="no-schedules">
                <p>No active schedules found.</p>
                <p>Create a daily or weekly schedule to see it here!</p>
            </div>
        `;
        return;
    }
    
    const schedulesHTML = schedules.map(schedule => {
        // Parse schedule ID to extract info if fields are missing
        let sport = schedule.sport || '';
        let league = schedule.league || '';
        let scheduleType = schedule.scheduleType || '';
        
        // Parse from schedule ID as fallback: collect-games-{type}-{sport}-{league}-{timestamp}
        if (!sport || !league || !scheduleType) {
            const parts = schedule.scheduleId.split('-');
            if (parts.length >= 5) {
                scheduleType = scheduleType || parts[2]; // daily or weekly
                sport = sport || parts[3];
                // Handle multi-word leagues like "college-football"
                if (parts.length > 6) {
                    league = league || parts.slice(4, -2).join('-');
                } else {
                    league = league || parts[4];
                }
            }
        }
        
        const teams = schedule.teams && schedule.teams.length > 0 ? schedule.teams.join(', ') : '';
        const conferences = schedule.conferences && schedule.conferences.length > 0 ? schedule.conferences.join(', ') : '';
        
        let tracking = 'All games';
        if (teams) {
            tracking = `Teams: ${teams}`;
        } else if (conferences) {
            tracking = `Conferences: ${conferences}`;
        }
        
        const nextRun = schedule.nextRunTime ? new Date(schedule.nextRunTime).toLocaleString() : 'N/A';
        const frequency = scheduleType === 'daily' ? 'Daily' : scheduleType === 'weekly' ? 'Weekly' : 'Scheduled';
        
        return `
        <div class="schedule-item">
            <div class="schedule-header">
                <div class="schedule-title">
                    ${sport.toUpperCase()} - ${league.toUpperCase()}
                    <div class="schedule-frequency">${frequency} at 1AM Eastern</div>
                </div>
                <div class="schedule-status ${schedule.paused ? 'paused' : 'active'}">
                    ${schedule.paused ? 'Paused' : 'Active'}
                </div>
            </div>
            <div class="schedule-details">
                <div><strong>Tracking:</strong> ${tracking}</div>
                <div><strong>Next Run:</strong> ${nextRun}</div>
                <div><strong>Schedule ID:</strong> ${schedule.scheduleId}</div>
            </div>
            <div class="schedule-actions">
                <button class="delete-btn" onclick="deleteSchedule('${schedule.scheduleId}')">Delete Schedule</button>
            </div>
        </div>
    `}).join('');
    
    schedulesList.innerHTML = schedulesHTML;
}

// Delete schedule
async function deleteSchedule(scheduleId) {
    if (!confirm('Are you sure you want to delete this schedule?')) {
        return;
    }
    
    try {
        await apiCall(`/api/schedules/${scheduleId}`, {
            method: 'DELETE'
        });
        
        showStatus('Schedule deleted successfully', 'success');
        loadSchedules();
    } catch (error) {
        showStatus('Failed to delete schedule', 'error');
    }
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
