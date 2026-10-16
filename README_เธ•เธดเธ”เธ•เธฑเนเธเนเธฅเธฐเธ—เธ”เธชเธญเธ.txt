DormHub STEP 15 - สมัครเจ้าของหอพักและอัปโหลดเอกสารจำลอง
=======================================================
โปรเจกต์นี้ยังเป็นระบบสาธิต ไม่ใช่ระบบพิสูจน์กรรมสิทธิ์จริง
ไม่ต้องใช้บัตรประชาชน ทะเบียนบ้าน โฉนด หรือหลักฐานส่วนบุคคลจริง

1. ก่อนแก้ไขไฟล์ สำรอง C:\Go Project\DormHub และ Export ฐานข้อมูล DormHub ผ่าน phpMyAdmin
2. เปิด XAMPP MySQL และ phpMyAdmin > ฐานข้อมูล DormHub > Import
   เลือก sql/Step15_Add_Landlord_Verification.sql
   (สร้างเพียงตารางใหม่ landlord_applications ไม่มี DROP/DELETE)
3. สำรอง main.go เดิม แล้ววาง main.go จาก ZIP ทับในโปรเจกต์
4. นำไฟล์ใน templates/ ทุกไฟล์วางใน C:\Go Project\DormHub\templates
5. นำ static/css/*.css ไปวางใน C:\Go Project\DormHub\static\css
   ไม่ต้องลบ static/uploads/dorms เดิม เพราะมีรูปหอพักที่อัปโหลดไว้
6. ไม่ต้องทับ go.mod / go.sum หากไฟล์เดิมใช้งานได้ตามปกติ
7. Ctrl+C เพื่อหยุดเว็บ แล้วเรียก go run main.go
8. เปิด /register > เลือก Landlord > กรอกข้อมูลตัวอย่าง และแนบ sample/เอกสารจำลอง_ทดสอบเจ้าของหอ.txt
9. Login บัญชีผู้สมัคร จะเห็น /landlord/application-status (pending)
10. Login เป็น Admin คนละบัญชี > เปิด /admin/landlord-applications
    ดาวน์โหลดไฟล์ตัวอย่าง กดอนุมัติหรือปฏิเสธ
11. Logout ผู้สมัครและ Login ใหม่ ถ้า approved ให้เข้า /landlord ได้

สำคัญ: ฐานข้อมูลยังชื่อ DormHub โดยไม่สร้างใหม่
role='admin' ไม่มีให้สมัครเอง ต้องใช้บัญชี Admin ที่คุณมีอยู่แล้ว
บัญชี landlord เดิมไม่ถูกปรับสิทธิ์หรือบังคับส่งเอกสารย้อนหลัง
เอกสารเก็บใน private_uploads/landlord_docs ไม่ได้ให้ดาวน์โหลดสาธารณะ
โฟลเดอร์ private_uploads จัดเก็บไว้กับโปรเจกต์สำหรับการทดลองในเครื่องเท่านั้น
ห้ามส่งไฟล์เอกสารส่วนตัวจริงขึ้นระบบสาธิต
