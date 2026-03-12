const API = '/api/v1';
let token = localStorage.getItem('token');
let username = localStorage.getItem('username');

if (token) showLoggedIn();
loadProducts();

function showMessage(text, type) {
    const el = document.getElementById('message');
    el.textContent = text;
    el.className = 'message ' + type;
    setTimeout(() => { el.className = 'message'; }, 3000);
}

async function api(path, options = {}) {
    if (token) {
        options.headers = { ...options.headers, 'Authorization': 'Bearer ' + token };
    }
    if (options.body) {
        options.headers = { ...options.headers, 'Content-Type': 'application/json' };
    }
    const resp = await fetch(API + path, options);
    return resp.json();
}

async function register() {
    const u = document.getElementById('username').value;
    const p = document.getElementById('password').value;
    if (!u || !p) { showMessage('Please fill in all fields', 'error'); return; }
    const res = await api('/auth/register', {
        method: 'POST',
        body: JSON.stringify({ username: u, password: p, email: u + '@example.com' })
    });
    if (res.code === 200) {
        showMessage('Registration successful! Please login.', 'success');
    } else {
        showMessage(res.message, 'error');
    }
}

async function login() {
    const u = document.getElementById('username').value;
    const p = document.getElementById('password').value;
    if (!u || !p) { showMessage('Please fill in all fields', 'error'); return; }
    const res = await api('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username: u, password: p })
    });
    if (res.code === 200) {
        token = res.data.token;
        username = res.data.user.username;
        localStorage.setItem('token', token);
        localStorage.setItem('username', username);
        showLoggedIn();
        showMessage('Login successful!', 'success');
        loadProducts();
    } else {
        showMessage(res.message, 'error');
    }
}

function logout() {
    token = null;
    username = null;
    localStorage.removeItem('token');
    localStorage.removeItem('username');
    document.getElementById('login-form').style.display = '';
    document.getElementById('user-info').style.display = 'none';
    document.getElementById('orders-section').style.display = 'none';
    loadProducts();
}

function showLoggedIn() {
    document.getElementById('login-form').style.display = 'none';
    document.getElementById('user-info').style.display = '';
    document.getElementById('user-display').textContent = username;
    document.getElementById('orders-section').style.display = '';
    loadOrders();
}

async function loadProducts() {
    const res = await api('/products?page=1&page_size=20');
    if (res.code !== 200) return;
    const list = document.getElementById('products-list');
    list.innerHTML = '';
    const items = res.data.items || [];
    if (items.length === 0) {
        list.innerHTML = '<p style="color:#888;">No products available.</p>';
        return;
    }
    items.forEach(p => {
        // Handle both plain Product and ProductDetailResponse formats
        const name = p.product ? p.product.name : p.name;
        const price = p.product ? p.product.price : p.price;
        const id = p.product ? p.product.id : p.id;
        const stock = p.inventory ? p.inventory.available : '';
        const stockText = stock !== '' ? `Stock: ${stock}` : '';
        list.innerHTML += `
            <div class="product-card">
                <div class="product-info">
                    <h3>${name}</h3>
                    <div class="price">&yen;${price.toFixed(2)}</div>
                    <div class="stock">${stockText}</div>
                </div>
                ${token ? `<button onclick="buyProduct(${id})">Buy Now</button>` : ''}
            </div>`;
    });
}

async function buyProduct(productId) {
    const res = await api('/orders', {
        method: 'POST',
        body: JSON.stringify({ product_id: productId, quantity: 1 })
    });
    if (res.code === 200) {
        showMessage('Order placed! No: ' + res.data.order_no, 'success');
        loadProducts();
        loadOrders();
    } else {
        showMessage(res.message, 'error');
    }
}

async function loadOrders() {
    if (!token) return;
    const res = await api('/orders?page=1&page_size=20');
    if (res.code !== 200) return;
    const list = document.getElementById('orders-list');
    list.innerHTML = '';
    const items = res.data.items || [];
    if (items.length === 0) {
        list.innerHTML = '<p style="color:#888;">No orders yet.</p>';
        return;
    }
    const statusMap = { 0: ['Pending', 'pending'], 1: ['Paid', 'paid'], 2: ['Cancelled', 'cancelled'] };
    items.forEach(o => {
        const [label, cls] = statusMap[o.status] || ['Unknown', 'pending'];
        list.innerHTML += `
            <div class="order-card">
                <div>
                    <strong>#${o.order_no}</strong>
                    <span> &yen;${o.total_price.toFixed(2)} x${o.quantity}</span>
                </div>
                <div>
                    <span class="status-${cls}">${label}</span>
                    ${o.status === 0 ? `<button onclick="cancelOrder(${o.id})" style="margin-left:1rem;background:#888;">Cancel</button>` : ''}
                </div>
            </div>`;
    });
}

async function cancelOrder(orderId) {
    const res = await api('/orders/' + orderId + '/cancel', { method: 'PUT' });
    if (res.code === 200) {
        showMessage('Order cancelled', 'success');
        loadProducts();
        loadOrders();
    } else {
        showMessage(res.message, 'error');
    }
}
