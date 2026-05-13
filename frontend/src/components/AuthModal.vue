<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

const open = defineModel('open', { type: Boolean, default: false })
const mode = defineModel('mode', { type: String, default: 'login' })
const auth = useAuthStore()
const form = reactive({ username: '', email: '', password: '', emailCode: '' })
const captcha = reactive({ challengeId: '', targetX: 0, trackWidth: 280, pieceWidth: 42, x: 0 })
const loading = ref(false)
const sendingCode = ref(false)
const securityOpen = ref(false)
const securityBusy = ref(false)
const pendingAction = ref('')
const error = ref('')
const notice = ref('')
let captchaTimer = null
let lastCaptchaX = 0

const captchaPassed = computed(() => Math.abs(captcha.x - captcha.targetX) <= 10)
const sliderMax = computed(() => Math.max(0, captcha.trackWidth - captcha.pieceWidth))
const targetStyle = computed(() => ({
  left: `${(captcha.targetX / captcha.trackWidth) * 100}%`,
  width: `${(captcha.pieceWidth / captcha.trackWidth) * 100}%`
}))
const pieceStyle = computed(() => ({
  left: `${(captcha.x / captcha.trackWidth) * 100}%`,
  width: `${(captcha.pieceWidth / captcha.trackWidth) * 100}%`
}))
const slideButtonStyle = computed(() => {
  const progress = sliderMax.value ? captcha.x / sliderMax.value : 0
  return { left: `calc(${progress * 100}% - ${progress * 54}px)` }
})

watch([open, mode], () => {
  error.value = ''
  notice.value = ''
  closeSecurity()
})

function switchMode(nextMode) {
  if (mode.value === nextMode) return
  mode.value = nextMode
  Object.assign(form, { username: '', email: '', password: '', emailCode: '' })
  error.value = ''
  notice.value = ''
  closeSecurity()
}

async function loadCaptcha() {
  const res = await api.post('/captcha/slide')
  captcha.challengeId = res.data.challenge_id
  captcha.targetX = res.data.target_x
  captcha.trackWidth = res.data.track_width
  captcha.pieceWidth = res.data.piece_width
  captcha.x = 0
  lastCaptchaX = 0
}

function handleCaptchaSlide(event) {
  if (securityBusy.value) return
  const nextX = Number(event.target.value || 0)
  const crossedTarget =
    (lastCaptchaX < captcha.targetX && nextX > captcha.targetX) ||
    (lastCaptchaX > captcha.targetX && nextX < captcha.targetX)
  const nearTarget = Math.abs(nextX - captcha.targetX) <= 14

  if (nearTarget || crossedTarget) {
    captcha.x = captcha.targetX
    lastCaptchaX = captcha.targetX
    scheduleSecurityFinish(180)
    return
  }

  captcha.x = nextX
  lastCaptchaX = nextX
}

function scheduleSecurityFinish(delay = 320) {
  if (captchaTimer) clearTimeout(captchaTimer)
  captchaTimer = setTimeout(() => {
    if (!captchaPassed.value || !securityOpen.value || securityBusy.value) return
    securityBusy.value = true
    finishSecurity()
  }, delay)
}

async function requestSecurity(action) {
  error.value = ''
  notice.value = ''
  pendingAction.value = action
  securityBusy.value = false
  securityOpen.value = true
  try {
    await loadCaptcha()
  } catch (err) {
    securityOpen.value = false
    error.value = err.message
  }
}

function closeSecurity() {
  if (captchaTimer) clearTimeout(captchaTimer)
  securityOpen.value = false
  securityBusy.value = false
  pendingAction.value = ''
  captcha.x = 0
  lastCaptchaX = 0
}

async function finishSecurity() {
  if (!captchaPassed.value) {
    securityBusy.value = false
    return
  }
  const action = pendingAction.value
  securityOpen.value = false
  pendingAction.value = ''
  if (action === 'email') {
    await sendEmailCodeWithCaptcha()
    return
  }
  if (action === 'submit') {
    await submitWithCaptcha()
  }
}

function sendEmailCode() {
  if (!form.email) {
    error.value = '请先填写邮箱'
    return
  }
  requestSecurity('email')
}

async function sendEmailCodeWithCaptcha() {
  sendingCode.value = true
  try {
    await api.post('/auth/email-code', {
      email: form.email,
      challenge_id: captcha.challengeId,
      captcha_x: Number(captcha.x)
    })
    notice.value = '验证码已发送，请查收邮箱'
  } catch (err) {
    error.value = err.message
  } finally {
    sendingCode.value = false
    securityBusy.value = false
  }
}

function submit() {
  requestSecurity('submit')
}

async function submitWithCaptcha() {
  loading.value = true
  try {
    if (mode.value === 'login') {
      await auth.login({
        email: form.email,
        password: form.password,
        challenge_id: captcha.challengeId,
        captcha_x: Number(captcha.x)
      })
      open.value = false
    } else {
      await auth.register({
        username: form.username,
        email: form.email,
        password: form.password,
        email_code: form.emailCode,
        challenge_id: captcha.challengeId,
        captcha_x: Number(captcha.x)
      })
      notice.value = '注册成功，账号已进入待审核状态'
      mode.value = 'login'
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
    securityBusy.value = false
  }
}
</script>

<template>
  <Transition name="modal-fade" appear>
    <div v-if="open" class="modal-backdrop auth-backdrop" @click.self="open = false">
      <div class="auth-card">
        <div class="modal-head">
          <div>
            <p class="section-kicker">{{ mode === 'login' ? 'Welcome back' : 'Create account' }}</p>
            <h2>{{ mode === 'login' ? '登录' : '注册' }}</h2>
          </div>
          <button class="icon-button" @click="open = false">×</button>
        </div>

        <div class="auth-tabs">
          <button :class="{ active: mode === 'login' }" @click="switchMode('login')">登录</button>
          <button :class="{ active: mode === 'register' }" @click="switchMode('register')">注册</button>
        </div>

        <form class="space-y-4" @submit.prevent="submit">
          <label v-if="mode === 'register'" class="field">
            <span>用户名</span>
            <input v-model="form.username" required />
          </label>
          <label class="field">
            <span>邮箱</span>
            <input v-model="form.email" type="email" required />
          </label>
          <label class="field">
            <span>密码</span>
            <input v-model="form.password" type="password" minlength="8" required />
          </label>

          <label v-if="mode === 'register'" class="field">
            <span>邮箱验证码</span>
            <div class="inline-field">
              <input v-model="form.emailCode" maxlength="6" required />
              <button class="ghost-button" type="button" :disabled="sendingCode" @click="sendEmailCode">
                {{ sendingCode ? '发送中' : '发送' }}
              </button>
            </div>
          </label>

          <div v-if="error" class="notice-card notice-error">
            <strong>操作未完成</strong>
            <span>{{ error }}</span>
          </div>
          <div v-if="notice" class="notice-card notice-success">
            <strong>处理成功</strong>
            <span>{{ notice }}</span>
          </div>
          <button class="primary-button w-full justify-center py-3" :disabled="loading">
            {{ loading ? '处理中' : mode === 'login' ? '登录' : '创建账号' }}
          </button>
        </form>
      </div>

      <Transition name="security-pop">
        <div v-if="securityOpen" class="security-backdrop" @click.self="closeSecurity">
          <section class="security-card security-dialog" :class="{ passed: captchaPassed }">
            <div class="security-head">
              <h3>请完成安全验证</h3>
              <button class="security-close" type="button" @click="closeSecurity">×</button>
            </div>
            <div class="captcha-stage">
              <div class="captcha-sky">
                <button class="security-refresh" type="button" title="刷新验证码" @click="loadCaptcha">↻</button>
                <span class="planet one"></span>
                <span class="planet two"></span>
                <span class="planet three"></span>
                <span class="planet four"></span>
                <span class="trace trace-one"></span>
                <span class="trace trace-two"></span>
                <span class="trace trace-three"></span>
                <span class="captcha-hole" :style="targetStyle"></span>
                <span class="captcha-fragment" :style="pieceStyle"></span>
              </div>
            </div>
            <div class="slide-rail">
              <input :value="captcha.x" type="range" min="0" :max="sliderMax" aria-label="拖动滑块完成验证" @input="handleCaptchaSlide" />
              <span class="slide-button" :style="slideButtonStyle">›</span>
            </div>
            <p class="security-tip">{{ captchaPassed ? '验证通过，正在继续操作' : '向右拖动滑块完成验证' }}</p>
          </section>
        </div>
      </Transition>
    </div>
  </Transition>
</template>
