---
Date: 05/04/2003
Summary: This is a brief description of the post.
Author: Kenton Vizdos
---

## What is Cloudflare Turnstile?

Cloudflare Turnstile is a CAPTCHA alternative designed to protect websites from spam, bots, and other automated threats without disrupting the user experience. Unlike traditional CAPTCHAs that require users to solve puzzles or identify images, Turnstile focuses on verifying the user’s authenticity through various browser signals (and, occasionally, will require a user to "check a box"). This results in a frictionless, more user-friendly interaction that is both secure and privacy-focused. With Turnstile, the goal is to eliminate the need for tedious verification challenges while still providing strong protection for your applications.

If you've ever seen one of the following pages, you've used Turnstile before:

![Cloudflare Turnstile on a Web Page](/assets/nested/test-image.jpg)
(credit: https://www.reddit.com/r/webscraping/comments/16hcde6/how_to_deal_with_cloudflare_human_verification/).

## What makes it so hard to use with LitJS / web components?
If you're anything like me, you LOVE web components for their simplicity and modularity. But if you’ve ever tried to add a Cloudflare Turnstile CAPTCHA to a Lit component, you’ve probably run into the dreaded "Error: document not found" message. Frustrating, right?

This issue arises because LitJS leverages the shadow DOM, which isolates component internals, including elements like Turnstile. In theory, a solution could be a shadow DOM mode for Turnstile, but after months of requests on the Cloudflare Discord, I decided it was time to find my own workaround.

## The solution? Slots.
Instead of trying to force Turnstile directly into the shadow DOM, the key is to use slots. By utilizing slots, we can bypass the isolation of the shadow DOM while still keeping our components neatly encapsulated.

If you're using something like the Vaadin router (or any router), this becomes a bit more "hacky," but once I remembered slots exist, the path forward became clear, even with more complex routing involved. This solution is especially helpful when using a router solution.

## Rendering Cloudflare Turnstile in the Shadow DOM

Well, technically we aren’t rendering it inside the shadow DOM—since we’re using a slot—but it still works, and I get the added benefit of keeping things SEO-friendly. Here's how to implement this:

In your index.html file, add the following script. This is the magic of the solution, so simple, yet so (eh, relatively) effective:

```html
<script>
  window.addEventListener('load-turnstile', e => {
    const script = document.createElement('script');
    script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js';
    script.async = true;
    document.body.appendChild(script);
    script.onload = () => {
      const currentComponent = e.detail.component;

      const widgetContainer = document.createElement('div');
      widgetContainer.setAttribute('slot', 'turnstile');
      const WidgetId = turnstile.render(widgetContainer, {
        sitekey: '3x00000000000000000000FF',
        callback: token => {
          const tokenEvent = new CustomEvent('turnstile-token', {
            detail: { token: token }, // Custom data to pass
          });
          currentComponent.dispatchEvent(tokenEvent);
        },
      });
      currentComponent.appendChild(widgetContainer);
    };
  });
</script>
```

[FYI: Find testing site keys here](https://google.com)
