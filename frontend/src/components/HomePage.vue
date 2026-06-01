<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'

gsap.registerPlugin(ScrollTrigger)

const props = defineProps({
  siteTitle: {
    type: String,
    default: '星空AI'
  },
  plans: {
    type: Array,
    default: () => []
  },
  allowRegistration: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['navigate', 'start'])

const homeRoot = ref(null)
let mm = null
let ctx = null

const sceneItems = [
  {
    eyebrow: 'Model Routing',
    title: '一次接入，稳定调用主流模型',
    body: '统一 OpenAI 兼容接口，覆盖 GPT 与 Claude，按业务需求在速度、成本和可用性之间平衡。',
    metric: '99.9%',
    metricLabel: '目标可用性',
    tone: 'cyan',
    stats: ['GPT', 'Claude', 'OpenAI API']
  },
  {
    eyebrow: 'Low Cost',
    title: '把每一次请求都变成可控成本',
    body: '透明额度、清晰单价和套餐周期，让个人开发、自动化脚本和团队实验都有稳定预算。',
    metric: '0.06',
    metricLabel: 'RMB = 1 USD',
    tone: 'amber',
    stats: ['日套餐', '周套餐', '公共额度']
  },
  {
    eyebrow: 'Asia Relay',
    title: '亚洲节点中转，响应更贴近开发现场',
    body: '面向中文开发者的服务链路，减少跨区不确定性，让调试、生成和批量任务更顺手。',
    metric: 'ms',
    metricLabel: '低延迟链路',
    tone: 'emerald',
    stats: ['快速响应', '稳定中转', '自动监控']
  },
  {
    eyebrow: 'Developer Console',
    title: '控制台、密钥、记录都在一个界面',
    body: '从购买套餐到创建 Key、查看用量和响应耗时，核心操作收束在同一套工作流里。',
    metric: '1',
    metricLabel: '站式管理',
    tone: 'violet',
    stats: ['API Key', '用量日志', '渠道状态']
  }
]

const activeScene = ref(0)
const visiblePlanCount = computed(() => props.plans.filter((plan) => Number(plan?.PriceCents || 0) >= 0).length)
const paidPlanCount = computed(() => props.plans.filter((plan) => Number(plan?.PriceCents || 0) > 0).length)
const minPlanPrice = computed(() => {
  const prices = props.plans
    .map((plan) => Number(plan?.PriceCents || 0))
    .filter((price) => price > 0)
    .sort((a, b) => a - b)
  return prices.length ? (prices[0] / 100).toFixed(2).replace(/\.?0+$/, '') : '0'
})

onMounted(async () => {
  await nextTick()
  setupAnimations()
})

onBeforeUnmount(() => {
  mm?.revert()
  ctx?.revert()
  mm = null
  ctx = null
})

function setupAnimations() {
  if (!homeRoot.value) return

  ctx = gsap.context(() => {
    mm = gsap.matchMedia()

    mm.add(
      {
        desktop: '(min-width: 900px)',
        mobile: '(max-width: 899px)',
        reduceMotion: '(prefers-reduced-motion: reduce)'
      },
      (context) => {
        const { desktop, reduceMotion } = context.conditions

        if (reduceMotion) {
          gsap.set('.home-animate, .cinema-visual, .scene-card', { clearProps: 'all' })
          return
        }

        gsap.timeline({ defaults: { ease: 'power3.out' } })
          .from('.home-animate', {
            y: 34,
            autoAlpha: 0,
            duration: 0.9,
            stagger: 0.08
          })
          .from('.hero-product-shell', {
            y: 46,
            rotationX: 10,
            scale: 0.94,
            autoAlpha: 0,
            duration: 1.05
          }, '<0.12')
          .from('.hero-orbit-line', {
            scaleX: 0,
            autoAlpha: 0,
            duration: 0.8,
            stagger: 0.08
          }, '<0.28')

        if (!desktop) {
          gsap.utils.toArray('.scene-card').forEach((card) => {
            gsap.from(card, {
              y: 42,
              autoAlpha: 0,
              duration: 0.8,
              ease: 'power3.out',
              scrollTrigger: {
                trigger: card,
                start: 'top 82%',
                toggleActions: 'play none none reverse'
              }
            })
          })
          return
        }

        const cards = gsap.utils.toArray('.scene-card')
        gsap.set(cards, { autoAlpha: 0, y: 38, scale: 0.96 })
        gsap.set(cards[0], { autoAlpha: 1, y: 0, scale: 1 })

        const tl = gsap.timeline({
          scrollTrigger: {
            trigger: '.cinema-scroll',
            start: 'top top',
            end: `+=${sceneItems.length * 780}`,
            pin: true,
            scrub: 0.9,
            snap: {
              snapTo: 1 / (sceneItems.length - 1),
              duration: 0.38,
              ease: 'power2.inOut'
            },
            onUpdate: (self) => {
              activeScene.value = Math.min(sceneItems.length - 1, Math.round(self.progress * (sceneItems.length - 1)))
            }
          }
        })

        cards.forEach((card, index) => {
          if (index === 0) return
          tl.to(cards[index - 1], { autoAlpha: 0, y: -38, scale: 0.96, duration: 0.45 }, index - 0.15)
            .to(card, { autoAlpha: 1, y: 0, scale: 1, duration: 0.55 }, index - 0.02)
            .to('.cinema-visual', {
              rotateY: index % 2 === 0 ? -10 : 10,
              rotateX: index % 2 === 0 ? 4 : -4,
              y: index % 2 === 0 ? -16 : 12,
              duration: 0.7
            }, index - 0.12)
            .to('.cinema-glow', {
              xPercent: index % 2 === 0 ? 12 : -12,
              yPercent: index % 2 === 0 ? -8 : 10,
              scale: 1 + index * 0.08,
              duration: 0.7
            }, '<')
        })

        gsap.to('.hero-product-shell', {
          y: -42,
          scale: 0.98,
          scrollTrigger: {
            trigger: '.home-hero-cinematic',
            start: 'top top',
            end: 'bottom top',
            scrub: 1
          }
        })
      },
      homeRoot.value
    )
  }, homeRoot.value)
}
</script>

<template>
  <main ref="homeRoot" class="home-cinematic">
    <section class="home-hero-cinematic">
      <div class="home-hero-grid mx-auto max-w-7xl px-4 sm:px-6">
        <div class="home-hero-copy">
          <p class="home-kicker home-animate">AI Gateway for Builders</p>
          <h1 class="home-animate">
            {{ siteTitle || '星空AI' }}
            <span>把顶级模型接入你的工作流</span>
          </h1>
          <p class="home-animate">
            面向中国开发者的一站式 AI API 服务。低成本额度、亚洲节点中转、统一模型接口和清晰控制台，让产品、脚本和团队工具稳定跑起来。
          </p>
          <div class="hero-actions home-animate">
            <button class="hero-primary" type="button" @click="emit('start')">
              <span>{{ allowRegistration ? '立即开始' : '登录使用' }}</span>
              <el-icon><ArrowRight /></el-icon>
            </button>
            <button class="hero-secondary" type="button" @click="emit('navigate', '/models')">
              <span>查看模型</span>
              <el-icon><Connection /></el-icon>
            </button>
          </div>
          <div class="hero-proof home-animate">
            <span><strong>{{ paidPlanCount || visiblePlanCount || 3 }}</strong> 套餐选择</span>
            <span><strong>￥{{ minPlanPrice }}</strong> 起步价格</span>
            <span><strong>GPT</strong> / Claude</span>
          </div>
        </div>

        <div class="hero-product-shell" aria-hidden="true">
          <div class="hero-product-topbar">
            <span></span><span></span><span></span>
            <strong>API Console</strong>
          </div>
          <div class="hero-product-body">
            <div class="hero-usage-ring">
              <span>$</span>
              <strong>128.6</strong>
              <small>available quota</small>
            </div>
            <div class="hero-console-stack">
              <div class="hero-console-row hot">
                <span>gpt-4o-mini</span>
                <strong>148ms</strong>
              </div>
              <div class="hero-console-row">
                <span>claude-sonnet</span>
                <strong>203ms</strong>
              </div>
              <div class="hero-console-row">
                <span>openai-compatible</span>
                <strong>ready</strong>
              </div>
            </div>
          </div>
          <div class="hero-orbit-line line-a"></div>
          <div class="hero-orbit-line line-b"></div>
        </div>
      </div>
    </section>

    <section class="cinema-scroll">
      <div class="cinema-shell mx-auto max-w-7xl px-4 sm:px-6">
        <div class="cinema-visual" aria-hidden="true">
          <div class="cinema-glow"></div>
          <div class="cinema-device">
            <div class="device-header">
              <span></span><span></span><span></span>
              <strong>{{ sceneItems[activeScene].eyebrow }}</strong>
            </div>
            <div class="device-stage" :class="`tone-${sceneItems[activeScene].tone}`">
              <div class="device-metric">
                <strong>{{ sceneItems[activeScene].metric }}</strong>
                <span>{{ sceneItems[activeScene].metricLabel }}</span>
              </div>
              <div class="device-bars">
                <i></i><i></i><i></i><i></i>
              </div>
              <div class="device-pills">
                <span v-for="tag in sceneItems[activeScene].stats" :key="tag">{{ tag }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="scene-stack">
          <article
            v-for="(item, index) in sceneItems"
            :key="item.eyebrow"
            class="scene-card"
            :class="[`tone-${item.tone}`, { active: activeScene === index }]"
          >
            <p>{{ item.eyebrow }}</p>
            <h2>{{ item.title }}</h2>
            <span>{{ item.body }}</span>
            <div class="scene-tags">
              <em v-for="tag in item.stats" :key="tag">{{ tag }}</em>
            </div>
          </article>
        </div>
      </div>
    </section>

    <section class="home-final-cta mx-auto max-w-7xl px-4 sm:px-6">
      <div>
        <p class="home-kicker">Ready</p>
        <h2>从一个 API Key 开始，把模型能力放进你的产品。</h2>
      </div>
      <div class="home-final-actions">
        <button class="hero-primary" type="button" @click="emit('start')">
          <span>进入控制台</span>
          <el-icon><ArrowRight /></el-icon>
        </button>
        <button class="hero-secondary" type="button" @click="emit('navigate', '/plans')">
          <span>查看价格</span>
          <el-icon><Money /></el-icon>
        </button>
      </div>
    </section>
  </main>
</template>

<style scoped>
.home-cinematic {
  overflow: hidden;
  background:
    linear-gradient(180deg, rgba(5, 8, 15, 0.98), rgba(8, 16, 26, 0.98) 46%, rgba(9, 14, 25, 0.98));
  color: #f8fafc;
}

.home-hero-cinematic {
  min-height: calc(100vh - 72px);
  display: grid;
  align-items: center;
  border-bottom: 1px solid rgba(148, 163, 184, 0.16);
  background:
    radial-gradient(ellipse at 72% 36%, rgba(45, 212, 191, 0.18), transparent 34%),
    radial-gradient(ellipse at 28% 18%, rgba(248, 113, 113, 0.16), transparent 32%),
    linear-gradient(180deg, rgba(8, 13, 23, 0.88), rgba(4, 8, 16, 0.98));
}

.home-hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(390px, 1fr);
  gap: clamp(34px, 6vw, 88px);
  align-items: center;
  padding-top: 64px;
  padding-bottom: 76px;
}

.home-hero-copy {
  display: grid;
  gap: 24px;
  align-content: center;
}

.home-kicker {
  margin: 0;
  color: #7dd3fc;
  font-size: 13px;
  font-weight: 950;
  letter-spacing: 0;
  text-transform: uppercase;
}

.home-hero-copy h1 {
  display: grid;
  gap: 14px;
  margin: 0;
  font-size: 76px;
  font-weight: 950;
  line-height: 0.98;
}

.home-hero-copy h1 span {
  max-width: 760px;
  color: rgba(248, 250, 252, 0.72);
  font-size: 52px;
}

.home-hero-copy > p:not(.home-kicker) {
  max-width: 680px;
  margin: 0;
  color: rgba(226, 232, 240, 0.78);
  font-size: 19px;
  line-height: 1.78;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 8px;
}

.hero-primary,
.hero-secondary {
  display: inline-flex;
  min-height: 54px;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border-radius: 999px;
  padding: 0 22px;
  font-size: 16px;
  font-weight: 950;
  transition:
    transform 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.hero-primary {
  border: 1px solid rgba(125, 211, 252, 0.42);
  background: linear-gradient(135deg, #f8fafc, #a7f3d0);
  color: #04111f;
  box-shadow: 0 24px 56px rgba(45, 212, 191, 0.22);
}

.hero-secondary {
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: rgba(15, 23, 42, 0.58);
  color: #f8fafc;
}

.hero-primary:hover,
.hero-secondary:hover {
  transform: translateY(-2px);
}

.hero-proof {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.hero-proof span {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.46);
  color: rgba(226, 232, 240, 0.72);
  padding: 10px 14px;
  font-size: 13px;
  font-weight: 800;
}

.hero-proof strong {
  color: #f8fafc;
  font-size: 18px;
}

.hero-product-shell {
  position: relative;
  min-height: 540px;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 8px;
  background:
    linear-gradient(160deg, rgba(15, 23, 42, 0.92), rgba(4, 8, 16, 0.96)),
    rgba(15, 23, 42, 0.9);
  box-shadow: 0 42px 140px rgba(0, 0, 0, 0.46);
  transform-style: preserve-3d;
  will-change: transform;
}

.hero-product-shell::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.06) 1px, transparent 1px),
    linear-gradient(180deg, rgba(255, 255, 255, 0.045) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: linear-gradient(180deg, black, transparent 84%);
}

.hero-product-topbar,
.device-header {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 52px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.14);
  padding: 0 18px;
}

.hero-product-topbar span,
.device-header span {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: rgba(148, 163, 184, 0.48);
}

.hero-product-topbar strong,
.device-header strong {
  margin-left: 8px;
  color: rgba(226, 232, 240, 0.72);
  font-size: 12px;
  font-weight: 950;
}

.hero-product-body {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: 0.9fr 1.1fr;
  gap: 18px;
  align-items: center;
  min-height: 488px;
  padding: 34px;
}

.hero-usage-ring {
  display: grid;
  width: min(270px, 100%);
  aspect-ratio: 1;
  place-items: center;
  justify-self: center;
  border: 1px solid rgba(125, 211, 252, 0.28);
  border-radius: 50%;
  background:
    conic-gradient(from 180deg, #67e8f9, #a7f3d0, #fbbf24, #67e8f9),
    rgba(15, 23, 42, 0.6);
  box-shadow: inset 0 0 0 28px rgba(4, 8, 16, 0.88), 0 24px 70px rgba(34, 211, 238, 0.24);
}

.hero-usage-ring span,
.hero-usage-ring small {
  color: rgba(226, 232, 240, 0.62);
  font-weight: 850;
}

.hero-usage-ring strong {
  margin-top: -34px;
  font-size: 66px;
  font-weight: 950;
}

.hero-console-stack {
  display: grid;
  gap: 14px;
}

.hero-console-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.72);
  padding: 18px 20px;
}

.hero-console-row.hot {
  border-color: rgba(125, 211, 252, 0.38);
  background: rgba(14, 116, 144, 0.2);
}

.hero-console-row span {
  color: rgba(226, 232, 240, 0.78);
  font-weight: 850;
}

.hero-console-row strong {
  color: #a7f3d0;
  font-weight: 950;
}

.hero-orbit-line {
  position: absolute;
  right: -8%;
  left: 44%;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(125, 211, 252, 0.74), transparent);
  transform-origin: left center;
}

.line-a {
  top: 30%;
  rotate: -18deg;
}

.line-b {
  bottom: 26%;
  rotate: 14deg;
}

.cinema-scroll {
  min-height: 100vh;
  background:
    radial-gradient(ellipse at 24% 30%, rgba(96, 165, 250, 0.12), transparent 34%),
    linear-gradient(180deg, #050815, #08101a);
}

.cinema-shell {
  display: grid;
  min-height: 100vh;
  grid-template-columns: minmax(380px, 1fr) minmax(340px, 0.72fr);
  gap: clamp(32px, 6vw, 86px);
  align-items: center;
  padding-top: 72px;
  padding-bottom: 72px;
}

.cinema-visual {
  position: relative;
  min-height: 560px;
  perspective: 1200px;
  transform-style: preserve-3d;
  will-change: transform;
}

.cinema-glow {
  position: absolute;
  inset: 8% 4% 6%;
  border-radius: 8px;
  background:
    linear-gradient(135deg, rgba(34, 211, 238, 0.26), rgba(167, 243, 208, 0.08), rgba(251, 191, 36, 0.14));
  filter: blur(42px);
  opacity: 0.72;
  will-change: transform;
}

.cinema-device {
  position: relative;
  z-index: 1;
  overflow: hidden;
  min-height: 560px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 8px;
  background: rgba(7, 12, 22, 0.94);
  box-shadow: 0 40px 120px rgba(0, 0, 0, 0.42);
}

.device-stage {
  display: grid;
  align-content: center;
  min-height: 508px;
  gap: 28px;
  padding: 38px;
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.045) 1px, transparent 1px),
    linear-gradient(180deg, rgba(255, 255, 255, 0.04) 1px, transparent 1px),
    radial-gradient(ellipse at 70% 20%, var(--scene-glow), transparent 46%);
  background-size: 40px 40px, 40px 40px, auto;
  transition: background-color 0.25s ease;
}

.tone-cyan {
  --scene-glow: rgba(34, 211, 238, 0.28);
  --scene-accent: #67e8f9;
}

.tone-amber {
  --scene-glow: rgba(251, 191, 36, 0.26);
  --scene-accent: #fbbf24;
}

.tone-emerald {
  --scene-glow: rgba(52, 211, 153, 0.26);
  --scene-accent: #86efac;
}

.tone-violet {
  --scene-glow: rgba(167, 139, 250, 0.25);
  --scene-accent: #c4b5fd;
}

.device-metric {
  display: grid;
  gap: 4px;
}

.device-metric strong {
  color: var(--scene-accent);
  font-size: 118px;
  font-weight: 950;
  line-height: 0.9;
}

.device-metric span {
  color: rgba(226, 232, 240, 0.72);
  font-size: 18px;
  font-weight: 900;
}

.device-bars {
  display: grid;
  gap: 12px;
}

.device-bars i {
  display: block;
  height: 16px;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--scene-accent), rgba(148, 163, 184, 0.14));
}

.device-bars i:nth-child(1) {
  width: 78%;
}

.device-bars i:nth-child(2) {
  width: 94%;
}

.device-bars i:nth-child(3) {
  width: 62%;
}

.device-bars i:nth-child(4) {
  width: 84%;
}

.device-pills,
.scene-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.device-pills span,
.scene-tags em {
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.56);
  color: rgba(248, 250, 252, 0.82);
  padding: 9px 12px;
  font-size: 12px;
  font-style: normal;
  font-weight: 900;
}

.scene-stack {
  position: relative;
  min-height: 430px;
}

.scene-card {
  position: absolute;
  inset: 0;
  display: grid;
  align-content: center;
  gap: 20px;
  border-left: 2px solid var(--scene-accent);
  padding-left: 30px;
  will-change: transform, opacity;
}

.scene-card p {
  margin: 0;
  color: var(--scene-accent);
  font-size: 13px;
  font-weight: 950;
  text-transform: uppercase;
}

.scene-card h2 {
  max-width: 520px;
  margin: 0;
  color: #f8fafc;
  font-size: 52px;
  font-weight: 950;
  line-height: 1.05;
}

.scene-card > span {
  max-width: 520px;
  color: rgba(226, 232, 240, 0.72);
  font-size: 18px;
  line-height: 1.78;
}

.home-final-cta {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 28px;
  align-items: center;
  padding-top: 86px;
  padding-bottom: 96px;
}

.home-final-cta h2 {
  max-width: 820px;
  margin: 10px 0 0;
  color: #f8fafc;
  font-size: 48px;
  font-weight: 950;
  line-height: 1.08;
}

.home-final-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: flex-end;
}

:global(html[data-theme="light"]) .home-cinematic {
  background: linear-gradient(180deg, #f8fbff, #eef5f8 48%, #f7fafc);
  color: #172033;
}

:global(html[data-theme="light"]) .home-hero-cinematic,
:global(html[data-theme="light"]) .cinema-scroll {
  background:
    radial-gradient(ellipse at 72% 36%, rgba(14, 165, 233, 0.16), transparent 34%),
    radial-gradient(ellipse at 28% 18%, rgba(245, 158, 11, 0.16), transparent 32%),
    linear-gradient(180deg, #f8fbff, #edf5f8);
}

:global(html[data-theme="light"]) .home-hero-copy h1,
:global(html[data-theme="light"]) .scene-card h2,
:global(html[data-theme="light"]) .home-final-cta h2 {
  color: #101828;
}

:global(html[data-theme="light"]) .home-hero-copy h1 span,
:global(html[data-theme="light"]) .home-hero-copy > p:not(.home-kicker),
:global(html[data-theme="light"]) .scene-card > span {
  color: rgba(23, 32, 51, 0.72);
}

:global(html[data-theme="light"]) .hero-secondary {
  border-color: rgba(15, 23, 42, 0.16);
  background: rgba(255, 255, 255, 0.72);
  color: #172033;
}

@media (max-width: 899px) {
  .home-hero-cinematic {
    min-height: auto;
  }

  .home-hero-grid,
  .cinema-shell,
  .home-final-cta {
    grid-template-columns: 1fr;
  }

  .home-hero-grid {
    padding-top: 42px;
    padding-bottom: 54px;
  }

  .home-hero-copy h1 {
    font-size: 46px;
  }

  .home-hero-copy h1 span {
    font-size: 34px;
  }

  .home-hero-copy > p:not(.home-kicker),
  .scene-card > span {
    font-size: 16px;
  }

  .hero-product-shell,
  .cinema-visual,
  .cinema-device {
    min-height: 420px;
  }

  .hero-product-body {
    grid-template-columns: 1fr;
    min-height: 368px;
    padding: 22px;
  }

  .hero-usage-ring {
    width: 210px;
  }

  .cinema-shell {
    min-height: auto;
    padding-top: 54px;
    padding-bottom: 54px;
  }

  .device-stage {
    min-height: 368px;
    padding: 24px;
  }

  .scene-stack {
    display: grid;
    min-height: 0;
    gap: 18px;
  }

  .scene-card {
    position: relative;
    min-height: 0;
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-left: 2px solid var(--scene-accent);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.52);
    padding: 24px;
  }

  .scene-card h2 {
    font-size: 34px;
  }

  .home-final-actions {
    justify-content: flex-start;
  }
}

@media (max-width: 560px) {
  .hero-actions,
  .home-final-actions {
    width: 100%;
  }

  .hero-primary,
  .hero-secondary {
    width: 100%;
  }

  .hero-proof span {
    width: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .hero-primary,
  .hero-secondary {
    transition: none;
  }
}
</style>
